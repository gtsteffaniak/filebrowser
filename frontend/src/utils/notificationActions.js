import { getters } from "@/store";
import { goToItem } from "@/utils/url";

export const GO_TO_ITEM_ACTION = "goToItem";

export function goToItemNotificationButton(label, source, path, isShare) {
  const share = isShare ?? getters.isShare();
  const resolvedSource = source ?? null;
  return {
    label,
    primary: true,
    actionType: GO_TO_ITEM_ACTION,
    actionData: { source: resolvedSource, path, isShare: share },
    action: () => goToItem(resolvedSource, path, {}, false, share),
  };
}

export function restoreNotificationButtonAction(button) {
  if (typeof button._action === "function") {
    return button._action;
  }
  if (typeof button.action === "function") {
    return button.action;
  }
  if (button.actionType === GO_TO_ITEM_ACTION && button.actionData?.path) {
    const { source, path, isShare } = button.actionData;
    return () => goToItem(source ?? null, path, {}, false, isShare ?? getters.isShare());
  }
  return null;
}

export function resolveHistoryNotificationButtons(buttons, activeButtons) {
  if (!buttons?.length) {
    return null;
  }

  const resolved = buttons
    .map((button, index) => {
      const activeButton = activeButtons?.[index];
      const action =
        (activeButton && typeof activeButton.action === "function" ? activeButton.action : null) ||
        restoreNotificationButtonAction(button);
      if (!action) {
        return null;
      }
      return { ...button, _action: action };
    })
    .filter(Boolean);

  return resolved.length ? resolved : null;
}
