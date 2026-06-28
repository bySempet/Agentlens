import { test, expect } from "@playwright/test";

// E2E del dashboard de trazas (E3) sobre datos de ejemplo (fixtures).

test("la lista de trazas muestra filas y navega al detalle", async ({ page }) => {
  await page.goto("/");
  await expect(page.getByRole("heading", { name: "Trazas" })).toBeVisible();

  const rows = page.getByTestId("trace-row");
  await expect(rows.first()).toBeVisible();
  expect(await rows.count()).toBeGreaterThanOrEqual(1);

  // Clic en la primera fila -> detalle de traza.
  await rows.first().click();
  await expect(page).toHaveURL(/\/traces\//);
});

test("el detalle muestra la conversación y el timeline", async ({ page }) => {
  await page.goto("/traces/9f2c1a7b3e");

  // Vista de conversación (E3-T04).
  await expect(page.getByTestId("conversation")).toBeVisible();
  expect(await page.locator('[data-testid="conversation"] .turn').count()).toBeGreaterThanOrEqual(1);

  // Timeline de spans (E3-T03).
  await expect(page.getByTestId("timeline")).toBeVisible();
  expect(await page.locator(".bar").count()).toBeGreaterThanOrEqual(1);
});

test("la vista de coste muestra agregados por agente y modelo", async ({ page }) => {
  await page.goto("/cost");
  await expect(page.getByRole("heading", { name: "Coste" })).toBeVisible();
  expect(await page.getByTestId("agent-cost").count()).toBeGreaterThanOrEqual(1);
  expect(await page.getByTestId("model-cost").count()).toBeGreaterThanOrEqual(1);
});
