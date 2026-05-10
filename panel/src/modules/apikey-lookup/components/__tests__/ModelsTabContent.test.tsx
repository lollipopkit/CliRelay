import { render, screen } from "@testing-library/react";
import { afterEach, describe, expect, test, vi } from "vitest";
import i18n from "@/i18n";
import { ModelsTabContent } from "@/modules/apikey-lookup/components/ModelsTabContent";
import { ThemeProvider } from "@/modules/ui/ThemeProvider";

describe("ModelsTabContent", () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  test("renders models and supports filtering", async () => {
    await i18n.changeLanguage("en");
    window.localStorage.clear();
    render(
      <ThemeProvider>
        <ModelsTabContent
          models={["claude-sonnet-4-5", "gpt-5.3-codex", "gemini-2.5-pro"]}
          loading={false}
          error={null}
          searchFilter=""
          onSearchChange={vi.fn()}
        />
      </ThemeProvider>,
    );

    expect(screen.getByText("claude-sonnet-4-5")).toBeInTheDocument();
    expect(screen.getByText("gpt-5.3-codex")).toBeInTheDocument();
    expect(screen.getByText("gemini-2.5-pro")).toBeInTheDocument();
    expect(screen.getByText("3")).toBeInTheDocument();
  });
});
