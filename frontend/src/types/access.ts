export type AccessRuleCategory = "user" | "group" | "all";

/** from backend access.FrontendRuleSet */
export interface AccessRuleSet {
  users: string[];
  groups: string[];
}

/** from backend access.FrontendAccessRule */
export interface AccessRule {
  denyAll?: boolean;
  deny: AccessRuleSet;
  allow: AccessRuleSet;
  sourceDenyDefault: boolean;
  pathExists: boolean;
}
