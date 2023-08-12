function closeSecurityWarning() {
  process.env.ELECTRON_DISABLE_SECURITY_WARNINGS = 'true'
}

export function setup() {
  closeSecurityWarning()
}
