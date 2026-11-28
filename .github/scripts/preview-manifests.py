#!/usr/bin/env python3
"""Render the chart the way the preview ApplicationSets do, and check it.

The previews deploy helm/inventario through three ApplicationSets in
infra/argocd/. Each names its overlay files and inline values, a project, and
the Pod Security labels of its namespace. This script reads those files, so a
check here follows the manifests instead of a copy of them.

  preview-manifests.py labels
      Print the Pod Security labels of the preview namespaces, one key=value
      per line, for `kubectl label`. Fails unless the three ApplicationSets
      carry the same labels and `enforce` is one of them.

  preview-manifests.py render OUT_DIR NAMESPACE
      Write the rendered chart of each ApplicationSet to OUT_DIR/<name>.yaml,
      for NAMESPACE.

  preview-manifests.py kinds
      Fail unless each AppProject allows exactly the kinds its Applications
      render: a kind the chart renders and the project refuses breaks that
      preview's sync, and a kind the project allows and nothing renders is a
      permission no preview needs.

The YAML here is read with patterns rather than a parser, because the CI
runners have no YAML library for Python. The patterns only have to match
files in this repository, and every lookup fails loudly when it finds nothing.
"""

import pathlib
import re
import subprocess
import sys
import tempfile

ROOT = pathlib.Path(__file__).resolve().parents[2]
ARGOCD = ROOT / "infra" / "argocd"
CHART = ROOT / "helm" / "inventario"

APPSETS = {
    "pr": "applicationset-pr.yaml",
    "master": "applicationset-master.yaml",
    "longevity": "applicationset-longevity.yaml",
}
PROJECT_FILES = ["appproject.yaml", "appproject-master.yaml"]


def fail(msg):
    print(f"error: {msg}", file=sys.stderr)
    sys.exit(1)


def block(text, key, path):
    """Return the lines nested under the first `key:` line, dedented."""
    lines = text.splitlines()
    for i, line in enumerate(lines):
        m = re.match(rf"^(\s*){re.escape(key)}:\s*(\|)?\s*$", line)
        if not m:
            continue
        indent = len(m.group(1))
        out = []
        for nested in lines[i + 1 :]:
            if nested.strip() and len(nested) - len(nested.lstrip()) <= indent:
                break
            out.append(nested)
        while out and not out[-1].strip():
            out.pop()
        if not out:
            fail(f"{path}: `{key}:` is empty")
        cut = min(len(x) - len(x.lstrip()) for x in out if x.strip())
        return [x[cut:] for x in out]
    fail(f"{path}: no `{key}:` found")


def pod_security_labels(name):
    path = ARGOCD / APPSETS[name]
    labels = block("\n".join(block(path.read_text(), "managedNamespaceMetadata", path)), "labels", path)
    pairs = []
    for line in labels:
        if not line.strip() or line.lstrip().startswith("#"):
            continue
        m = re.match(r"^([\w./-]+):\s*(\S+)\s*$", line)
        if not m:
            fail(f"{path}: cannot read label line {line!r}")
        pairs.append((m.group(1), m.group(2).strip("'\"")))
    if not any(k == "pod-security.kubernetes.io/enforce" for k, _ in pairs):
        fail(f"{path}: managedNamespaceMetadata sets no pod-security.kubernetes.io/enforce")
    return sorted(pairs)


def cmd_labels():
    want = pod_security_labels("pr")
    for name in APPSETS:
        got = pod_security_labels(name)
        if got != want:
            fail(f"{APPSETS[name]} has namespace labels {got}, {APPSETS['pr']} has {want}")
    for k, v in want:
        print(f"{k}={v}")


def appset(name):
    """Return (value files, inline values, project) of one ApplicationSet."""
    path = ARGOCD / APPSETS[name]
    text = path.read_text()
    files = re.findall(r"^\s*- \$values/(\S+)\s*$", text, re.M)
    if not files:
        fail(f"{path}: no `$values/` value files")
    inline = "\n".join(block(text, "values", path)) + "\n"
    # ApplicationSet template fields and the bootstrap.sh placeholder. Any
    # value that is valid in a name, a host and an image tag does.
    inline = re.sub(r"\{\{[^}]*\}\}", "0000000", inline).replace("<TAILNET>", "tailnet")
    project = re.search(r"^\s+project:\s*(\S+)\s*$", text, re.M)
    if not project:
        fail(f"{path}: no `project:`")
    return files, inline, project.group(1)


def render(name, namespace, workdir):
    files, inline, _ = appset(name)
    values = pathlib.Path(workdir) / f"{name}-inline-values.yaml"
    values.write_text(inline)
    # The release name is the Application name, which the chart's object
    # names are built from.
    release = "inv-vcl01-pr0000000" if name == "pr" else f"inv-vcl01-{name}"
    args = ["helm", "template", release, str(CHART), "--namespace", namespace]
    for f in files:
        args += ["-f", str(ROOT / f)]
    args += ["-f", str(values)]
    return subprocess.run(args, check=True, capture_output=True, text=True).stdout


def rendered_kinds(manifest):
    kinds = set()
    for doc in re.split(r"^---\s*$", manifest, flags=re.M):
        api = re.search(r"^apiVersion:\s*(\S+)\s*$", doc, re.M)
        kind = re.search(r"^kind:\s*(\S+)\s*$", doc, re.M)
        if api and kind:
            group = api.group(1).rpartition("/")[0]
            kinds.add((group, kind.group(1)))
    if not kinds:
        fail("the rendered chart has no objects")
    return kinds


def project_kinds():
    projects = {}
    for file in PROJECT_FILES:
        path = ARGOCD / file
        text = path.read_text()
        name = re.search(r"^metadata:\s*\n\s+name:\s*(\S+)\s*$", text, re.M)
        if not name:
            fail(f"{path}: no metadata.name")
        entries = block(text, "namespaceResourceWhitelist", path)
        kinds = set(
            (g.strip("'\""), k)
            for g, k in re.findall(r"^- group:\s*(\S*)\s*\n\s+kind:\s*(\S+)\s*$", "\n".join(entries), re.M)
        )
        if not kinds:
            fail(f"{path}: cannot read namespaceResourceWhitelist")
        projects[name.group(1)] = kinds
    return projects


def cmd_kinds():
    projects = project_kinds()
    needed = {}
    with tempfile.TemporaryDirectory() as workdir:
        for name in APPSETS:
            project = appset(name)[2]
            if project not in projects:
                fail(f"{APPSETS[name]} uses project {project!r}, which no AppProject in infra/argocd defines")
            needed.setdefault(project, set()).update(rendered_kinds(render(name, f"inv-vcl01-{name}", workdir)))
    ok = True
    for project, allowed in sorted(projects.items()):
        rendered = needed.get(project, set())
        for group, kind in sorted(rendered - allowed):
            print(f"error: the chart renders {group or 'core'}/{kind}, which AppProject {project} does not allow", file=sys.stderr)
            ok = False
        for group, kind in sorted(allowed - rendered):
            print(f"error: AppProject {project} allows {group or 'core'}/{kind}, which none of its Applications renders", file=sys.stderr)
            ok = False
        names = [f"{group or 'core'}/{kind}" for group, kind in sorted(rendered)]
        print(f"{project}: {', '.join(names)}")
    if not ok:
        sys.exit(1)


def main(argv):
    if argv[1:] == ["labels"]:
        cmd_labels()
    elif argv[1:] == ["kinds"]:
        cmd_kinds()
    elif len(argv) == 4 and argv[1] == "render":
        out = pathlib.Path(argv[2])
        out.mkdir(parents=True, exist_ok=True)
        with tempfile.TemporaryDirectory() as workdir:
            for name in APPSETS:
                (out / f"{name}.yaml").write_text(render(name, argv[3], workdir))
    else:
        fail("usage: preview-manifests.py labels | kinds | render OUT_DIR NAMESPACE")


if __name__ == "__main__":
    main(sys.argv)
