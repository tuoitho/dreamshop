export const ADMIN_CONSOLE_PERMISSIONS = [
  "product.view",
  "product.create",
  "product.update",
  "product.delete",
  "inventory.view",
  "inventory.update",
  "user.view",
  "user.status.update",
  "user.role.assign",
  "rbac.view",
  "rbac.manage",
] as const

export type AdminPermission = (typeof ADMIN_CONSOLE_PERMISSIONS)[number]

export function hasAnyAdminPermission(permissions: ReadonlySet<string> | readonly string[]): boolean {
  const set = permissions instanceof Set ? permissions : new Set(permissions)
  return ADMIN_CONSOLE_PERMISSIONS.some((perm) => set.has(perm))
}
