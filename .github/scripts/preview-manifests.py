#!/usr/bin/env python3
"""Render the chart the way the preview ApplicationSets do, and check it.

The previews deploy helm/inventario through three ApplicationSets in
infra/argocd/. Each names its overlay files and inline values, a project, and
the Pod Security labels of its namespace. This script reads those files, so a
check here follows the manifests instead of a copy of them.

  preview-manifests.py labels
      Print the Pod Security labels of the preview namespaces, one key=value
      per line, for `kubectl label`. Fails unless the three ApplicationSets
      carry the same Pod Security labels and `enforce` is one of them.

  preview-manifests.py render OUT_DIR NAMESPACE
      Write the rendered chart of each ApplicationSet to OUT_DIR/<name>.yaml,
      for NAMESPACE.

  preview-manifests.py kinds
      Fail unless each AppProject allows exactly the kinds its Applications
      render: a kind the chart renders and the project refuses breaks that
      preview's sync, and a kind the project allows and nothing renders is a
      permission no preview needs.

  preview-manifests.py limits [MANIFEST]
      Fail unless the PR preview render stays within the caps of
      infra/vm/cluster-extras/preview-namespace-limits.yaml: its
      ResourceQuota sets every capped resource at or below the cap, its
      LimitRange defaults and maximums are at or below the per-container
      ceiling, and every container and init container has a CPU and a
      memory limit at or below the ceiling, set on the container or filled
      in by a LimitRange default. A render past them would have its quota or
      its pods refused on the cluster. Also fails unless the PR
      ApplicationSet puts the label that the pod policy selects its
      namespaces by on the namespace, and the namespace guard requires it.
      With MANIFEST, checks that file instead of rendering the chart.

The YAML here is read with patterns rather than a parser, because the CI
runners have no YAML library for Python. The patterns only have to match
files in this repository, and every lookup fails loudly when it finds nothing.
"""

import fractions
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
LIMITS_POLICY = ROOT / "infra" / "vm" / "cluster-extras" / "preview-namespace-limits.yaml"
NAMESPACE_GUARD = ROOT / "infra" / "vm" / "cluster-extras" / "preview-namespace-pod-security.yaml"
POD_SECURITY_PREFIX = "pod-security.kubernetes.io/"
WORKLOAD_KINDS = ("Deployment", "Job", "StatefulSet", "DaemonSet", "ReplicaSet", "CronJob", "Pod")

QUANTITY_SUFFIXES = {
    "": 1, "m": fractions.Fraction(1, 1000),
    "k": 10**3, "M": 10**6, "G": 10**9, "T": 10**12,
    "Ki": 2**10, "Mi": 2**20, "Gi": 2**30, "Ti": 2**40,
}


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


def namespace_labels(name):
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
    return sorted(pairs)


def pod_security_labels(name):
    pairs = [(k, v) for k, v in namespace_labels(name) if k.startswith(POD_SECURITY_PREFIX)]
    if not any(k == "pod-security.kubernetes.io/enforce" for k, _ in pairs):
        fail(f"{APPSETS[name]}: managedNamespaceMetadata sets no pod-security.kubernetes.io/enforce")
    return pairs


def cmd_labels():
    want = pod_security_labels("pr")
    for name in APPSETS:
        got = pod_security_labels(name)
        if got != want:
            fail(f"{APPSETS[name]} has Pod Security labels {got}, {APPSETS['pr']} has {want}")
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


def quantity(text, where):
    m = re.fullmatch(r"([0-9]+(?:\.[0-9]+)?)(m|k|M|G|T|Ki|Mi|Gi|Ti)?", text.strip("'\""))
    if not m:
        fail(f"{where}: cannot read quantity {text!r}")
    return fractions.Fraction(m.group(1)) * QUANTITY_SUFFIXES[m.group(2) or ""]


def policy_caps():
    """Return (quota caps, per-container ceiling) from the limits policy."""
    text = LIMITS_POLICY.read_text()
    caps_expr = re.search(r"- name: caps\s*\n\s*expression: >-\s*\n\s*(\{[^}]*\})", text)
    if not caps_expr:
        fail(f"{LIMITS_POLICY}: no `caps` variable")
    caps = dict(re.findall(r"'([^']+)':\s*'([^']+)'", caps_expr.group(1)))
    ceiling = dict(re.findall(r"limits\['(cpu|memory)'\]\)\.compareTo\(quantity\('([^']+)'\)\)", text))
    if not caps or set(ceiling) != {"cpu", "memory"}:
        fail(f"{LIMITS_POLICY}: cannot read the quota caps and the per-container ceiling")
    return caps, ceiling


def documents(manifest, kind):
    return [d for d in re.split(r"^---\s*$", manifest, flags=re.M) if re.search(rf"^kind:\s*{kind}\s*$", d, re.M)]


def nested_pairs(lines, start):
    """Return the `key: value` lines nested under lines[start], as a dict."""
    indent = len(lines[start]) - len(lines[start].lstrip())
    pairs = {}
    for line in lines[start + 1 :]:
        if line.strip() and len(line) - len(line.lstrip()) <= indent:
            break
        m = re.match(r"^\s*([\w./-]+):\s*(\S+)\s*$", line)
        if m:
            pairs[m.group(1)] = m.group(2)
    return pairs


def flow_or_nested(lines, start):
    """Return the mapping under lines[start], block or `{a: b}` flow style."""
    m = re.match(r"^\s*(?:- )?[\w./-]+:\s*\{(.*)\}\s*$", lines[start])
    if m:
        return dict(
            (k.strip(), v.strip().strip("'\""))
            for k, v in (pair.split(":", 1) for pair in m.group(1).split(",") if ":" in pair)
        )
    return nested_pairs(lines, start)


def sequence_items(lines, start):
    """Return the items of the block sequence under lines[start].

    Each item is a list of lines whose first line has its `- ` replaced by
    spaces, so every key of the item sits at the same indentation.
    """
    key_indent = len(lines[start]) - len(lines[start].lstrip())
    items, item_indent = [], None
    for line in lines[start + 1 :]:
        stripped = line.lstrip()
        if not stripped or stripped.startswith("#"):
            if items:
                items[-1].append(line)
            continue
        indent = len(line) - len(stripped)
        if item_indent is None:
            if indent < key_indent or not stripped.startswith("- "):
                break
            item_indent = indent
        if indent < item_indent or (indent == item_indent and not stripped.startswith("- ")):
            break
        if indent == item_indent:
            items.append([" " * (indent + 2) + stripped[2:]])
        else:
            items[-1].append(line)
    return items


def child(lines, key):
    """Return the index of `key:` at the item's own indentation, or None."""
    indent = len(lines[0]) - len(lines[0].lstrip())
    for i, line in enumerate(lines):
        if len(line) - len(line.lstrip()) == indent and re.match(rf"^\s*{re.escape(key)}:", line):
            return i
    return None


def container_limits(doc):
    """Yield (container, init, explicit limits) for every container of a workload."""
    lines = doc.splitlines()
    for i, line in enumerate(lines):
        m = re.match(r"^\s*(containers|initContainers):\s*$", line)
        if not m:
            continue
        for item in sequence_items(lines, i):
            n = child(item, "name")
            name = item[n].split(":", 1)[1].strip().strip("'\"") if n is not None else "?"
            limits = {}
            r = child(item, "resources")
            if r is not None:
                resources = item[r:]
                indent = len(resources[0]) - len(resources[0].lstrip())
                end = next((j for j, l in enumerate(resources[1:], 1)
                            if l.strip() and len(l) - len(l.lstrip()) <= indent), len(resources))
                resources = resources[:end]
                for j, l in enumerate(resources):
                    if re.match(r"^\s*limits:", l):
                        limits = flow_or_nested(resources, j)
            yield name, m.group(1) == "initContainers", limits


def limit_range_values(manifest):
    """Return (defaults, maximums) of the Container LimitRanges in the render."""
    defaults, maximums = {}, {}
    for doc in documents(manifest, "LimitRange"):
        lines = doc.splitlines()
        start = next((i for i, l in enumerate(lines) if re.match(r"^  limits:\s*$", l)), None)
        if start is None:
            continue
        for item in sequence_items(lines, start):
            t = child(item, "type")
            if t is None or item[t].split(":", 1)[1].strip().strip("'\"") != "Container":
                continue
            values = {}
            for key in ("default", "max"):
                k = child(item, key)
                values[key] = flow_or_nested(item, k) if k is not None else {}
            # The API server fills an unset default from max.
            for res, value in {**values["max"], **values["default"]}.items():
                defaults.setdefault(res, []).append(value)
            for res, value in values["max"].items():
                maximums.setdefault(res, []).append(value)
    return defaults, maximums


def preview_label_problems():
    """Check that the PR namespaces carry the label the pod policy selects by."""
    text = LIMITS_POLICY.read_text()
    selector = re.search(r"namespaceSelector:\s*\n\s*matchLabels:\s*\n\s*([\w./-]+):\s*(\S+)\s*$", text, re.M)
    if not selector:
        return [f"{LIMITS_POLICY}: no namespaceSelector on the pod policy"]
    key, value = selector.group(1), selector.group(2).strip("'\"")
    problems = []
    if (key, value) not in namespace_labels("pr"):
        problems.append(f"{APPSETS['pr']} does not label its namespaces {key}: {value}, which the pod policy selects")
    guard = NAMESPACE_GUARD.read_text()
    if not re.search(rf"labels\['{re.escape(key)}'\] == '{re.escape(value)}'", guard):
        problems.append(f"{NAMESPACE_GUARD.name} does not require {key}: {value} on PR namespaces")
    return problems


def cmd_limits(manifest_path=None):
    caps, ceiling = policy_caps()
    if manifest_path:
        manifest = pathlib.Path(manifest_path).read_text()
    else:
        with tempfile.TemporaryDirectory() as workdir:
            manifest = render("pr", "inv-vcl01-pr1", workdir)
    problems = preview_label_problems()

    quotas = documents(manifest, "ResourceQuota")
    if len(quotas) != 1:
        problems.append(f"the PR render has {len(quotas)} ResourceQuotas, expected 1")
    for doc in quotas:
        lines = doc.splitlines()
        hard = nested_pairs(lines, next(i for i, l in enumerate(lines) if l.strip() == "hard:"))
        for key, cap in sorted(caps.items()):
            if key not in hard:
                problems.append(f"the ResourceQuota sets no {key}; the cluster requires it at most {cap}")
            elif quantity(hard[key], "ResourceQuota") > quantity(cap, LIMITS_POLICY):
                problems.append(f"the ResourceQuota sets {key}: {hard[key]}, above the cap of {cap}")

    defaults, maximums = limit_range_values(manifest)
    for res, top in ceiling.items():
        for value in maximums.get(res, []) + defaults.get(res, []):
            if quantity(value, "LimitRange") > quantity(top, LIMITS_POLICY):
                problems.append(f"the LimitRange sets a {res} limit of {value}, above the ceiling of {top}")

    checked = 0
    for kind in WORKLOAD_KINDS:
        for doc in documents(manifest, kind):
            workload = re.search(r"^  name:\s*(\S+)\s*$", doc, re.M).group(1)
            for name, init, limits in container_limits(doc):
                checked += 1
                what = f"{kind} {workload}, {'init container' if init else 'container'} {name}"
                for res, top in sorted(ceiling.items()):
                    values = [limits[res]] if res in limits else defaults.get(res, [])
                    if not values:
                        problems.append(f"{what} has no {res} limit, and no LimitRange default supplies one")
                    for value in values:
                        if quantity(value, what) > quantity(top, LIMITS_POLICY):
                            problems.append(f"{what} gets a {res} limit of {value}, above the ceiling of {top}")
    if checked == 0:
        problems.append("found no containers in the PR render")

    for p in problems:
        print(f"error: {p}", file=sys.stderr)
    if problems:
        sys.exit(1)
    print(f"PR preview within the caps: quota {', '.join(f'{k} <= {v}' for k, v in sorted(caps.items()))}; "
          f"{checked} containers with CPU and memory limits <= {ceiling['cpu']} CPU / {ceiling['memory']}")


def main(argv):
    if argv[1:] == ["labels"]:
        cmd_labels()
    elif argv[1:] == ["kinds"]:
        cmd_kinds()
    elif argv[1:2] == ["limits"] and len(argv) <= 3:
        cmd_limits(argv[2] if len(argv) == 3 else None)
    elif len(argv) == 4 and argv[1] == "render":
        out = pathlib.Path(argv[2])
        out.mkdir(parents=True, exist_ok=True)
        with tempfile.TemporaryDirectory() as workdir:
            for name in APPSETS:
                (out / f"{name}.yaml").write_text(render(name, argv[3], workdir))
    else:
        fail("usage: preview-manifests.py labels | kinds | limits [MANIFEST] | render OUT_DIR NAMESPACE")


if __name__ == "__main__":
    main(sys.argv)
