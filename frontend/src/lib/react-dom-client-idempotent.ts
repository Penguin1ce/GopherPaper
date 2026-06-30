import type * as ReactDOMClientTypes from "react-dom/client";
// Turbopack aliases "react-dom/client" to this file, so the runtime import must
// bypass package exports and point at the real implementation.
// @ts-expect-error react-dom does not publish declarations for the physical file.
import * as RealReactDOMClient from "../../node_modules/react-dom/client.js";

const ROOTS_KEY = "__gopherpaperReactRoots";

const ReactDOMClient = RealReactDOMClient as typeof ReactDOMClientTypes;

type RootRegistry = WeakMap<object, ReturnType<typeof ReactDOMClientTypes.createRoot>>;

function rootRegistry(): RootRegistry {
  const scope = globalThis as typeof globalThis & {
    [ROOTS_KEY]?: RootRegistry;
  };
  return scope[ROOTS_KEY] ?? (scope[ROOTS_KEY] = new WeakMap());
}

export const createRoot: typeof ReactDOMClientTypes.createRoot = (container, options) => {
  const roots = rootRegistry();
  const existing = roots.get(container);
  if (existing) return existing;
  const root = ReactDOMClient.createRoot(container, options);
  const unmount = root.unmount.bind(root);
  root.unmount = () => {
    roots.delete(container);
    unmount();
  };
  roots.set(container, root);
  return root;
};

export const hydrateRoot = ReactDOMClient.hydrateRoot;
