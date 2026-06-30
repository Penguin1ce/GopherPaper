import type * as ReactDOMClientTypes from "react-dom/client";
// Next 16/Turbopack runs React through its compiled canary pair in dev.
// Import that matching ReactDOM entry instead of the physical npm react-dom.
// @ts-expect-error Next does not publish declarations for compiled internals.
import * as RealReactDOMClient from "next/dist/compiled/react-dom/client";

const ROOTS_KEY = "__gopherpaperReactRoots";
const ROOT_KEY = "__gopherpaperReactRoot";

const ReactDOMClient = RealReactDOMClient as typeof ReactDOMClientTypes;

type Root = ReturnType<typeof ReactDOMClientTypes.createRoot>;
type RootRegistry = WeakMap<object, Root>;
type RootContainer = Parameters<typeof ReactDOMClientTypes.createRoot>[0] & {
  [ROOT_KEY]?: Root | null;
};

function rootRegistry(): RootRegistry {
  const scope = globalThis as typeof globalThis & {
    [ROOTS_KEY]?: RootRegistry;
  };
  return scope[ROOTS_KEY] ?? (scope[ROOTS_KEY] = new WeakMap());
}

function containerRoot(container: RootContainer): Root | undefined {
  return rootRegistry().get(container) ?? container[ROOT_KEY] ?? undefined;
}

function rememberRoot(container: RootContainer, root: Root) {
  rootRegistry().set(container, root);
  Object.defineProperty(container, ROOT_KEY, {
    configurable: true,
    value: root,
    writable: true,
  });
}

function forgetRoot(container: RootContainer, root: Root) {
  if (rootRegistry().get(container) === root) {
    rootRegistry().delete(container);
  }
  if (container[ROOT_KEY] === root) {
    delete container[ROOT_KEY];
  }
}

export const createRoot: typeof ReactDOMClientTypes.createRoot = (container, options) => {
  const rootContainer = container as RootContainer;
  const existing = containerRoot(rootContainer);
  if (existing) return existing;

  const root = ReactDOMClient.createRoot(container, options);
  const unmount = root.unmount.bind(root);
  root.unmount = () => {
    try {
      unmount();
    } finally {
      forgetRoot(rootContainer, root);
    }
  };
  rememberRoot(rootContainer, root);
  return root;
};

export const hydrateRoot = ReactDOMClient.hydrateRoot;
