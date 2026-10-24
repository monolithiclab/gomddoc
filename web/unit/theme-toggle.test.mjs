// theme-toggle.mjs runs on import against the current document, so each test
// builds the page first and imports a fresh copy of the module.
import { beforeEach, describe, expect, it, vi } from "vitest";

async function load(button) {
  document.body.innerHTML = button;
  vi.resetModules();
  await import("../../cmd/gomddoc/assets/shared/theme-toggle.mjs");
  return document.getElementById("theme-toggle");
}

beforeEach(() => {
  localStorage.clear();
  document.documentElement.setAttribute("data-theme", "light");
});

describe("theme toggle", () => {
  it("fills an empty button with a line icon, not an emoji", async () => {
    const toggle = await load('<button id="theme-toggle"></button>');
    expect(toggle.querySelector("svg path")).not.toBeNull(); // the moon
    expect(toggle.textContent.trim()).toBe("");

    toggle.click();
    expect(document.documentElement.getAttribute("data-theme")).toBe("dark");
    expect(toggle.querySelector("svg circle")).not.toBeNull(); // the sun
    expect(toggle.textContent.trim()).toBe("");
  });

  it("shows only the child icon for the current theme", async () => {
    const toggle = await load(
      '<button id="theme-toggle"><svg data-show-theme="light"></svg><svg data-show-theme="dark"></svg></button>',
    );
    const shown = () =>
      [...toggle.querySelectorAll("svg")]
        .filter((svg) => svg.style.display !== "none")
        .map((svg) => svg.dataset.showTheme);
    expect(shown()).toEqual(["light"]);
    toggle.click();
    expect(shown()).toEqual(["dark"]);
  });

  it("uses the data-light / data-dark labels when given", async () => {
    const toggle = await load(
      '<button id="theme-toggle" data-light="Dark Mode" data-dark="Light Mode"></button>',
    );
    expect(toggle.textContent).toBe("Dark Mode");
    toggle.click();
    expect(toggle.textContent).toBe("Light Mode");
  });
});
