const LEVELS: Record<number, string> = {
  5: "role_minimal",
  10: "role_guest",
  20: "role_reporter",
  30: "role_developer",
  40: "role_maintainer",
  50: "role_owner",
};

/** 把 GitLab access_level 映射成 i18n key。 */
export function accessLevelKey(level: number): string {
  return LEVELS[level] ?? "role_unknown";
}
