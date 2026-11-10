// The product's existing browser runner discovers this owner-local contribution.
export async function run({page,expect,successfulObject}) {
  await page.goto('audit')
  await expect(page.locator('#audit-since-filter')).toBeVisible()
  await expect(page.locator('#audit-until-filter')).toBeVisible()
  await successfulObject(await page.request.get('api/security/audit?limit=25'))
  await page.goto('diagnostics')
  await expect(page.locator('.diagnostics')).toBeVisible()
  await expect(page.getByText('Log inspector',{exact:true})).toBeVisible()
  await expect(page.getByRole('button',{name:'Export bundle',exact:true})).toBeVisible()
  await successfulObject(await page.request.get('api/diagnostics/report'))
  await successfulObject(await page.request.get('api/logs/entries?count=10'))
}
