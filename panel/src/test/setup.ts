import "@testing-library/jest-dom/vitest";
import { afterEach } from "vitest";
import { cleanup } from "@testing-library/react";
import "@/i18n";

if (typeof globalThis !== "undefined") {
  const createMemoryStorage = () => {
    const values = new Map<string, string>();
    return {
      getItem: (key: string) => values.get(key) ?? null,
      setItem: (key: string, value: string) => {
        values.set(String(key), String(value));
      },
      removeItem: (key: string) => {
        values.delete(String(key));
      },
      clear: () => {
        values.clear();
      },
      key: (index: number) => {
        return Array.from(values.keys())[index] ?? null;
      },
      get length() {
        return values.size;
      },
      _values: values,
    };
  };

  const isStorageUsable = (value: unknown): value is Storage => {
    return (
      typeof value === "object" &&
      value !== null &&
      typeof (value as Storage).getItem === "function" &&
      typeof (value as Storage).setItem === "function" &&
      typeof (value as Storage).removeItem === "function" &&
      typeof (value as Storage).clear === "function" &&
      typeof (value as Storage).key === "function"
    );
  };

  if (!isStorageUsable((globalThis as Record<string, unknown>).localStorage)) {
    const storage = createMemoryStorage();
    Object.defineProperty(globalThis, "localStorage", {
      configurable: true,
      value: storage,
      writable: true,
    });
  }

  if (typeof window !== "undefined" && !isStorageUsable(window.localStorage)) {
    const storage = createMemoryStorage();
    Object.defineProperty(window, "localStorage", {
      configurable: true,
      value: storage,
      writable: true,
    });
  }
}

afterEach(() => {
  cleanup();
});

if (typeof window !== "undefined") {
  if (!window.matchMedia) {
    window.matchMedia = ((query: string) =>
      ({
        matches: false,
        media: query,
        onchange: null,
        addListener: () => undefined,
        removeListener: () => undefined,
        addEventListener: () => undefined,
        removeEventListener: () => undefined,
        dispatchEvent: () => false,
      }) as unknown as MediaQueryList) as typeof window.matchMedia;
  }
}

if (typeof globalThis !== "undefined" && !(globalThis as any).ResizeObserver) {
  (globalThis as any).ResizeObserver = class ResizeObserver {
    observe() {
      // noop
    }
    unobserve() {
      // noop
    }
    disconnect() {
      // noop
    }
  };
}
