import { render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, test, vi } from "vitest";
import i18n from "@/i18n";
import { SystemPage } from "@/modules/system/SystemPage";
import { ThemeProvider } from "@/modules/ui/ThemeProvider";
import { ToastProvider } from "@/modules/ui/ToastProvider";

const mocks = vi.hoisted(() => ({
  apiGet: vi.fn(),
}));

vi.mock("@/lib/http/client", () => ({
  apiClient: {
    get: mocks.apiGet,
  },
}));

vi.mock("@/modules/auth/AuthProvider", () => ({
  useAuth: () => ({
    state: {
      apiBase: "http://localhost:8317",
      serverVersion: "main-1111111",
      serverBuildDate: "2026-04-16T08:00:00Z",
    },
    meta: {
      managementEndpoint: "/v0/management",
    },
  }),
}));

function renderPage() {
  return render(
    <ThemeProvider>
      <ToastProvider>
        <SystemPage />
      </ToastProvider>
    </ThemeProvider>,
  );
}

describe("SystemPage", () => {
  beforeEach(async () => {
    await i18n.changeLanguage("en");
    window.localStorage.clear();
    mocks.apiGet.mockImplementation((path: string) => {
      if (path === "/models") return Promise.resolve({ data: [] });
      if (path === "/model-configs?scope=library") return Promise.resolve({ data: [] });
      if (path === "/auth-files") return Promise.resolve({ files: [] });
      if (
        path === "/gemini-api-key" ||
        path === "/claude-api-key" ||
        path === "/codex-api-key" ||
        path === "/vertex-api-key" ||
        path === "/openai-compatibility"
      ) {
        return Promise.resolve([]);
      }
      if (path === "/system-stats") return Promise.resolve({ uptime: 10 });
      return Promise.resolve({});
    });
  });

  test("loads and renders system overview", async () => {
    renderPage();
    await waitFor(() => {
      expect(screen.getByRole("heading", { level: 2, name: /system info/i })).toBeInTheDocument();
    });
    expect(screen.getByText("Available Models")).toBeInTheDocument();
  });

  test("uses auth-file model owner groups instead of raw registry models", async () => {
    window.localStorage.setItem(
      "authFilesPage.modelOwnerGroupMap.v1",
      JSON.stringify({ claude: "anthropic" }),
    );
    mocks.apiGet.mockImplementation((path: string) => {
      if (path === "/models") {
        return Promise.resolve({
          data: [{ id: "claude-ghost-model", type: "claude", disabled: false }],
        });
      }
      if (path === "/auth-files") {
        return Promise.resolve({
          files: [{ name: "claude-account.json", type: "claude", disabled: false }],
        });
      }
      if (path === "/model-configs?scope=library") {
        return Promise.resolve({
          data: [
            {
              id: "claude-3-7-sonnet-latest",
              owned_by: "anthropic",
              description: "Mapped Claude model",
              enabled: true,
            },
          ],
        });
      }
      if (
        path === "/gemini-api-key" ||
        path === "/claude-api-key" ||
        path === "/codex-api-key" ||
        path === "/vertex-api-key" ||
        path === "/openai-compatibility"
      ) {
        return Promise.resolve([]);
      }
      if (path === "/system-stats") return Promise.resolve({ uptime: 10 });
      return Promise.resolve({});
    });

    renderPage();

    expect(await screen.findByText("claude-3-7-sonnet-latest")).toBeInTheDocument();
    expect(screen.queryByText("claude-ghost-model")).not.toBeInTheDocument();
  });
});
