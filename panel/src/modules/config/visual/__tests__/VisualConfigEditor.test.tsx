import { act, fireEvent, render, renderHook, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, test, vi } from "vitest";
import i18n from "@/i18n";
import { VisualConfigEditor } from "@/modules/config/visual/VisualConfigEditor";
import { DEFAULT_VISUAL_VALUES } from "@/modules/config/visual/types";
import { useVisualConfig } from "@/modules/config/visual/useVisualConfig";
import { ThemeProvider } from "@/modules/ui/ThemeProvider";

function renderEditor(onChange = vi.fn()) {
  render(
    <ThemeProvider>
      <VisualConfigEditor
        values={{
          ...DEFAULT_VISUAL_VALUES,
        }}
        onChange={onChange}
      />
    </ThemeProvider>,
  );
  return onChange;
}

describe("VisualConfigEditor", () => {
  beforeEach(async () => {
    await i18n.changeLanguage("en");
  });

  test("exposes browser CORS origins as one origin per line", async () => {
    const onChange = renderEditor();

    const textarea = screen.getByRole("textbox", { name: /cors allowed origins/i });
    fireEvent.change(textarea, {
      target: {
        value: "chrome-extension://abcdefghijklmnop\nhttp://localhost:5173",
      },
    });

    expect(onChange).toHaveBeenLastCalledWith({
      corsAllowOriginsText: "chrome-extension://abcdefghijklmnop\nhttp://localhost:5173",
    });
  });

  test("loads and writes cors allow origins in config yaml", async () => {
    const { result } = renderHook(() => useVisualConfig());

    act(() => {
      result.current.loadVisualValuesFromYaml(
        [
          "cors-allow-origins:",
          "  - https://admin.example.com",
          "  - chrome-extension://abcdefghijklmnop",
        ].join("\n"),
      );
    });

    await waitFor(() => {
      expect(result.current.visualValues.corsAllowOriginsText).toBe(
        "https://admin.example.com\nchrome-extension://abcdefghijklmnop",
      );
    });

    act(() => {
      result.current.setVisualValues({
        corsAllowOriginsText:
          " https://plugin.example \n\nchrome-extension://abcdefghijklmnop\nhttps://plugin.example",
      });
    });

    await waitFor(() => {
      const nextYaml = result.current.applyVisualChangesToYaml("");
      expect(nextYaml).toContain("cors-allow-origins:");
      expect(nextYaml).toContain("- https://plugin.example");
      expect(nextYaml).toContain("- chrome-extension://abcdefghijklmnop");
      expect(nextYaml.match(/https:\/\/plugin\.example/g)).toHaveLength(1);
    });
  });
});
