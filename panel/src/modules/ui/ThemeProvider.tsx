import {
  createContext,
  type PropsWithChildren,
  use,
  useCallback,
  useEffect,
  useMemo,
  useState,
} from "react";
import { flushSync } from "react-dom";
import { Moon, Sun } from "lucide-react";
import {
  DEFAULT_HIGHLIGHT_COLOR,
  HIGHLIGHT_COLOR_STORAGE_KEY,
  THEME_STORAGE_KEY,
} from "@/lib/constants";

export type ThemeMode = "light" | "dark";
export type HighlightColor = `#${string}`;

interface ThemeContextState {
  state: {
    mode: ThemeMode;
    accentColor: HighlightColor;
  };
  actions: {
    setMode: (mode: ThemeMode) => void;
    toggle: () => void;
    setAccentColor: (color: string) => void;
  };
}

interface ThemeColors {
  accent: HighlightColor;
  hover: HighlightColor;
  active: HighlightColor;
}

const ThemeContext = createContext<ThemeContextState | null>(null);

const readThemeSnapshot = (): ThemeMode | null => {
  try {
    const raw = localStorage.getItem(THEME_STORAGE_KEY);
    if (raw === "dark" || raw === "light") {
      return raw;
    }
    return null;
  } catch {
    return null;
  }
};

const normalizeHexColor = (input: string): HighlightColor | null => {
  if (!input) return null;
  const trimmed = input.trim().toLowerCase();
  if (!/^#([0-9a-f]{6})$/.test(trimmed)) return null;
  return `#${trimmed.slice(1)}` as HighlightColor;
};

const readAccentSnapshot = (): HighlightColor => {
  try {
    const raw = localStorage.getItem(HIGHLIGHT_COLOR_STORAGE_KEY);
    const normalized = raw ? normalizeHexColor(raw) : null;
    return normalized ?? DEFAULT_HIGHLIGHT_COLOR;
  } catch {
    return DEFAULT_HIGHLIGHT_COLOR;
  }
};

const resolveSystemTheme = (): ThemeMode => {
  if (typeof window === "undefined") {
    return "light";
  }
  return window.matchMedia?.("(prefers-color-scheme: dark)")?.matches ? "dark" : "light";
};

const blendWithBase = (color: HighlightColor, target: number, amount: number): HighlightColor => {
  const hex = color.slice(1);
  const base = Number.parseInt(hex, 16);
  const baseR = (base >> 16) & 0xff;
  const baseG = (base >> 8) & 0xff;
  const baseB = base & 0xff;
  const blendedR = Math.round(baseR + (target - baseR) * amount);
  const blendedG = Math.round(baseG + (target - baseG) * amount);
  const blendedB = Math.round(baseB + (target - baseB) * amount);
  const toHex = (value: number): string =>
    Math.min(255, Math.max(0, value)).toString(16).padStart(2, "0");
  return `#${toHex(blendedR)}${toHex(blendedG)}${toHex(blendedB)}` as HighlightColor;
};

const hexToRgbString = (color: HighlightColor): string => {
  const hex = color.slice(1);
  const base = Number.parseInt(hex, 16);
  return `${(base >> 16) & 0xff},${(base >> 8) & 0xff},${base & 0xff}`;
};

const getThemeColors = (accent: HighlightColor): ThemeColors => ({
  accent,
  hover: blendWithBase(accent, 255, 0.12),
  active: blendWithBase(accent, 0, 0.18),
});

const applyThemeToDom = (mode: ThemeMode): void => {
  document.documentElement.classList.toggle("dark", mode === "dark");
  document.documentElement.setAttribute("data-theme", mode);
};

const applyAccentToDom = (accent: HighlightColor): void => {
  const root = document.documentElement;
  const colors = getThemeColors(accent);
  root.style.setProperty("--highlight-color", colors.accent);
  root.style.setProperty("--primary-color", colors.accent);
  root.style.setProperty("--primary-color-rgb", hexToRgbString(accent));
  root.style.setProperty("--primary-hover", colors.hover);
  root.style.setProperty("--primary-active", colors.active);
};

const persistTheme = (mode: ThemeMode): void => {
  localStorage.setItem(THEME_STORAGE_KEY, mode);
};

const persistAccent = (accent: HighlightColor): void => {
  localStorage.setItem(HIGHLIGHT_COLOR_STORAGE_KEY, accent);
};

const runWithViewTransition = (fn: () => void) => {
  const startViewTransition = document.startViewTransition;
  if (typeof startViewTransition !== "function") {
    fn();
    return;
  }
  try {
    startViewTransition(() => {
      flushSync(fn);
    });
  } catch {
    fn();
  }
};

export function ThemeProvider({ children }: PropsWithChildren) {
  const [mode, setModeState] = useState<ThemeMode>(() => readThemeSnapshot() ?? resolveSystemTheme());
  const [accentColor, setAccentColorState] = useState<HighlightColor>(() => readAccentSnapshot());

  useEffect(() => {
    applyThemeToDom(mode);
    applyAccentToDom(accentColor);
    persistTheme(mode);
    persistAccent(accentColor);
  }, [mode, accentColor]);

  const setMode = useCallback((next: ThemeMode) => {
    applyThemeToDom(next);
    persistTheme(next);
    runWithViewTransition(() => setModeState(next));
  }, []);

  const setAccentColor = useCallback((next: string) => {
    const normalized = normalizeHexColor(next);
    if (!normalized) {
      return;
    }
    const nextAccent = normalized.toUpperCase() as HighlightColor;
    applyAccentToDom(nextAccent);
    persistAccent(nextAccent);
    setAccentColorState(nextAccent);
  }, []);

  const toggle = useCallback(() => {
    setMode(mode === "dark" ? "light" : "dark");
  }, [mode, setMode]);

  const value = useMemo<ThemeContextState>(
    () => ({
      state: { mode, accentColor },
      actions: { setMode, toggle, setAccentColor },
    }),
    [accentColor, mode, setAccentColor, setMode, toggle],
  );

  return <ThemeContext value={value}>{children}</ThemeContext>;
}

export const useTheme = (): ThemeContextState => {
  const context = use(ThemeContext);
  if (!context) {
    throw new Error("useTheme must be used within ThemeProvider");
  }
  return context;
};

export function ThemeToggleButton({ className, label }: { className?: string; label?: string }) {
  const {
    state: { mode },
    actions: { toggle },
  } = useTheme();

  const Icon = mode === "dark" ? Sun : Moon;
  const text = label ?? (mode === "dark" ? "Switch to light" : "Switch to dark");

  return (
    <button type="button" onClick={toggle} className={className} aria-label={text} title={text}>
      <Icon size={16} />
    </button>
  );
}
