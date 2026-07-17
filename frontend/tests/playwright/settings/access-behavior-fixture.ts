export type AccessRuleExpectation = {
    path: string;
    denyTotal: number;
    allowTotal: number;
};

/**
 * Access rules on the "access" source after Bolt migration plus
 * Dockerfile.playwright-settings `filebrowser set rule` commands.
 * Root `/` keeps the migrated deny-basic rule and gains allow-admin from the Dockerfile.
 */
export const ACCESS_SOURCE_RULES_AFTER_DOCKER_SETUP: AccessRuleExpectation[] = [
    { path: "/", allowTotal: 1, denyTotal: 1 },
    { path: "/denied/", allowTotal: 0, denyTotal: 1 },
    { path: "/excluded/", allowTotal: 0, denyTotal: 1 },
    { path: "/excluded/showme.txt/", allowTotal: 1, denyTotal: 0 },
];

export function sortAccessRules(rules: AccessRuleExpectation[]): AccessRuleExpectation[] {
    return [...rules].sort((a, b) => a.path.localeCompare(b.path));
}
