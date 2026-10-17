export function load({ params }) {
  /** @type {Record<string, string>} */
  const labels = { tasks: 'Tasks', settings: 'Settings', git: 'Git', audit: 'Audit' };
  return { label: labels[params.path] ?? 'Manager', phase: 2 };
}
