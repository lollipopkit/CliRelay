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
          autoUpdateEnabled: true,
          autoUpdateChannel: "main",
        }}
        onChange={onChange}
      />
    </ThemeProvider>,
  );
  return onChange;
}

describe("VisualConfigEditor auto update config", () => {
  beforeEach(async () => {
    await i18n.changeLanguage("en");
  });

  test("shows automatic update settings and exposes main/dev source branches", async () => {
    const onChange = renderEditor();

    const toggle = screen.getByRole("switch", { name: /automatic update checks/i });
    await userEvent.click(toggle);
    expect(onChange).toHaveBeenCalledWith({ autoUpdateEnabled: false });

    const select = screen.getByRole("combobox", { name: /update source branch/i });
    await userEvent.click(select);
    expect(screen.queryByRole("option", { name: /auto-detect/i })).not.toBeInTheDocument();
    await userEvent.click(await screen.findByRole("option", { name: /development/i }));

    expect(onChange).toHaveBeenCalledWith({ autoUpdateChannel: "dev" });
  });

  test("loads and writes auto-update settings in config yaml", async () => {
    const { result } = renderHook(() => useVisualConfig());

    act(() => {
      result.current.loadVisualValuesFromYaml(
        "auto-update:\n  enabled: false\n  channel: dev\n  docker-image: registry.local/mirror/clirelay\n",
      );
    });

    await waitFor(() => {
      expect(result.current.visualValues).toMatchObject({
        autoUpdateEnabled: false,
        autoUpdateChannel: "dev",
      });
    });

    act(() => {
      result.current.setVisualValues({
        autoUpdateEnabled: true,
        autoUpdateChannel: "dev",
      });
    });

    await waitFor(() => {
      const nextYaml = result.current.applyVisualChangesToYaml("");
      expect(nextYaml).toContain("auto-update:");
      expect(nextYaml).toContain("enabled: true");
      expect(nextYaml).toContain("channel: dev");
      expect(nextYaml).not.toContain("docker-image:");
    });
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
