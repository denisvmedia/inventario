import { File as NodeFile } from "node:buffer"

// Uploads in the jsdom test environment send a FormData built from jsdom's
// FormData and File, but the request goes through Node's fetch, which only
// understands Node's own classes. Vitest's jsdom environment converts the
// body, and the conversion loses every file's name and bytes
// (vitest-dev/vitest#11587): it copies each jsdom Blob into a Node Blob with
// no filename, and Node's FormData then wraps that Blob in the global File,
// which in this environment is jsdom's and does not take the bytes. msw 2
// passed the resulting empty part on. msw 3 fails the request with
// RequestContentLengthMismatchError instead.
//
// installNodeFormDataFetch wraps the global fetch (the one msw patched, so it
// has to run after server.listen()) and rebuilds such a body from Node's own
// FormData and File before msw sees it. A handler then receives the real
// multipart body: field names, file names, types and bytes. It returns a
// function that puts the previous fetch back.
//
// Remove this once Vitest's own conversion keeps the files (#11587).

let nodeFormDataClass: Promise<typeof FormData> | undefined

// Node exposes its FormData class only as a global, and the jsdom environment
// replaces that global. A FormData that Node's own Response parses is an
// instance of Node's class.
function nodeFormData(): Promise<typeof FormData> {
  nodeFormDataClass ??= new Response(new URLSearchParams())
    .formData()
    .then((form) => form.constructor as typeof FormData)
  return nodeFormDataClass
}

async function toNodeFormData(form: FormData): Promise<FormData> {
  const NodeFormData = await nodeFormData()
  const out = new NodeFormData()
  for (const [key, value] of form.entries()) {
    if (typeof value === "string") {
      out.append(key, value)
      continue
    }
    const file = new NodeFile([await value.arrayBuffer()], value.name, {
      type: value.type,
      lastModified: value.lastModified,
    })
    out.append(key, file as unknown as File)
  }
  return out
}

export function installNodeFormDataFetch(): () => void {
  const inner = globalThis.fetch
  const wrapped: typeof fetch = async (input, init) => {
    // `FormData` here is jsdom's class; a body already built from Node's
    // classes is passed through untouched.
    if (init?.body instanceof FormData) {
      return inner(input, { ...init, body: await toNodeFormData(init.body) })
    }
    return inner(input, init)
  }
  globalThis.fetch = wrapped
  return () => {
    if (globalThis.fetch === wrapped) globalThis.fetch = inner
  }
}
