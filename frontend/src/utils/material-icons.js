// Material Symbols icons that don't exist in Material Icons
// These need to use the 'material-symbols-outlined' class
// Note: This is not exhaustive - Material Symbols has 2000+ icons
// We list common ones that users might search for
const materialSymbolsOnly = new Set([
  // Development & Code
  'deployed_code',
  'deployed_code_alert',
  'code_blocks',
  'javascript',
  'css',
  'html',
  'terminal',
  'sdk',
  'api',
  'webhook',
  'commit',
  'robot',
  'code_off',
  'php',
  'python',
  'ruby',
  'java',
  'cpp',
  'typescript',

  // Design & Creative
  'design_services',
  'web_stories',
  'markdown',
  'animated_images',
  'gif_box',
  'stacks',
  'package_2',
  'instant_mix',

  // Smart Home & IoT
  'nest_farsight_eco',
  'nest_cam_wired_stand',
  'nest_doorbell_visitor',
  'nest_protect',
  'nest_thermostat',
  'nest_wifi_router',
  'nest_clock_farsight_analog',
  'nest_clock_farsight_digital',
  'nest_remote',
  'nest_true_radiant',
  'nest_wake_on_approach',
  'nest_wake_on_press',
  'nest_display',
  'nest_eco_leaf',
  'nest_heat_link',
  'nest_sunblock',

  // Data & Analytics
  'demography',
  'counter_1',
  'counter_2',
  'counter_3',
  'counter_4',
  'counter_5',
  'counter_6',
  'counter_7',
  'counter_8',
  'counter_9',

  // Media & Content
  'movie_edit',
  'audio_video_receiver',
  'manga',
  'sports_score',
  'rainy_light',
  'rainy_snow',
  'rainy_heavy',

  // UI & Navigation
  'splitscreen_vertical_add',
  'splitscreen_left',
  'splitscreen_right',
  'collapse_content',
  'expand_content',
  'keep',
  'keep_off',

  // Communication
  'chat_add_on',
  'forums',
  'news',
  'subscriptions',

  // Misc modern icons
  'deployed_code_history',
  'deployed_code_update',
  'passkey',
  'home_app_logo',
  'assistant',
  'assistant_on_hub',
]);

/**
 * Get the appropriate icon class for an icon name
 * @param {string} iconName - The material icon name
 * @returns {string} - The CSS class to use ('material-icons' or 'material-symbols-outlined')
 */
const getIconClass = (iconName) => {
  return materialSymbolsOnly.has(iconName) ? 'material-symbols-outlined' : 'material-icons';
};

// Comprehensive list of popular Material Icons
const materialIcons = [
  // File & Folder
  'folder', 'folder_open', 'insert_drive_file', 'description', 'article', 'note',
  'create_new_folder', 'folder_shared', 'folder_special', 'topic',

  // Actions
  'add', 'remove', 'edit', 'delete', 'save', 'close', 'check', 'clear',
  'done', 'done_all', 'undo', 'redo', 'refresh', 'sync', 'settings',
  'more_vert', 'more_horiz', 'menu', 'search', 'filter_list',

  // Navigation
  'home', 'arrow_back', 'arrow_forward', 'arrow_upward', 'arrow_downward',
  'chevron_left', 'chevron_right', 'expand_more', 'expand_less',
  'first_page', 'last_page', 'fullscreen', 'fullscreen_exit', 'open_in_new',

  // Media
  'play_arrow', 'pause', 'stop', 'skip_next', 'skip_previous',
  'volume_up', 'volume_down', 'volume_off', 'image', 'photo', 'camera',
  'videocam', 'movie', 'music_note', 'audiotrack', 'album',

  // Communication
  'email', 'mail', 'message', 'chat', 'comment', 'forum', 'feedback',
  'phone', 'call', 'contact_phone', 'contacts', 'person', 'people',
  'group', 'share', 'send', 'forward', 'reply',

  // Content
  'content_copy', 'content_cut', 'content_paste', 'link', 'attachment',
  'text_format', 'format_bold', 'format_italic', 'format_underlined',
  'format_list_bulleted', 'format_list_numbered', 'insert_link',

  // Alerts & Status
  'info', 'help', 'warning', 'error', 'check_circle', 'cancel',
  'notifications', 'notification_important', 'priority_high',

  // Device & System
  'computer', 'laptop', 'phone_iphone', 'tablet', 'watch', 'tv',
  'storage', 'cloud', 'cloud_upload', 'cloud_download', 'cloud_done',
  'wifi', 'bluetooth', 'battery_full', 'battery_charging_full',

  // Places & Time
  'place', 'map', 'location_on', 'public', 'language', 'explore',
  'schedule', 'today', 'date_range', 'event', 'alarm', 'timer',
  'access_time', 'history', 'update',

  // Toggle & Controls
  'star', 'star_border', 'favorite', 'favorite_border', 'visibility',
  'visibility_off', 'lock', 'lock_open', 'verified_user', 'security',
  'bookmark', 'bookmark_border', 'label', 'label_outline',

  // Shopping & Business
  'shopping_cart', 'shopping_bag', 'store', 'payment', 'credit_card',
  'account_balance', 'work', 'business', 'domain', 'apartment',

  // Social
  'thumb_up', 'thumb_down', 'thumb_up_off_alt', 'thumb_down_off_alt',
  'sentiment_satisfied', 'sentiment_dissatisfied', 'mood', 'mood_bad',

  // Miscellaneous
  'dashboard', 'widgets', 'apps', 'extension', 'color_lens', 'palette',
  'brush', 'build', 'construction', 'handyman', 'bug_report',
  'code', 'developer_mode', 'integration_instructions', 'analytics',
  'bar_chart', 'pie_chart', 'trending_up', 'trending_down',
  'interests', 'category', 'grade', 'sports_esports', 'casino',
];

// Popular Material Symbols (newer icons not in Material Icons)
const materialSymbols = [
  // Development & Code
  'deployed_code', 'deployed_code_alert', 'code_blocks', 'javascript', 'css',
  'html', 'terminal', 'sdk', 'api', 'webhook', 'commit', 'robot', 'code_off',
  'php', 'python', 'ruby', 'java', 'cpp', 'typescript',

  // Design & Creative
  'design_services', 'web_stories', 'markdown', 'animated_images',
  'gif_box', 'stacks', 'package_2', 'instant_mix',

  // Smart Home & IoT
  'nest_farsight_eco', 'nest_cam_wired_stand', 'nest_doorbell_visitor',
  'nest_protect', 'nest_thermostat', 'nest_wifi_router', 'nest_display',

  // Data & Analytics
  'demography',
  'counter_1', 'counter_2', 'counter_3', 'counter_4', 'counter_5',
  'counter_6', 'counter_7', 'counter_8', 'counter_9',

  // Media & Content
  'movie_edit', 'audio_video_receiver', 'manga', 'sports_score',

  // UI & Navigation
  'splitscreen_vertical_add', 'splitscreen_left', 'splitscreen_right',
  'collapse_content', 'expand_content', 'keep', 'keep_off',

  // Communication
  'chat_add_on', 'forums', 'news', 'subscriptions',

  // Misc modern icons
  'passkey', 'assistant',
];

// Combined list of all available icons
const allMaterialIcons = [...materialIcons, ...materialSymbols].sort();

export {
  materialSymbolsOnly,
  getIconClass,
  materialIcons,
  materialSymbols,
  allMaterialIcons,
};

