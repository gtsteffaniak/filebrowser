// Material Symbols icons that don't exist in Material Icons
// These need to use the 'material-symbols-outlined' class
// Note: Many newer icons are Symbols-only. This list includes known Symbols-only icons.
const materialSymbolsOnly = new Set([
  // Development & Code
  'deployed_code', 'deployed_code_alert', 'deployed_code_history', 'deployed_code_update',
  'code_blocks', 'javascript', 'css', 'html', 'terminal', 'sdk', 'api', 'webhook',
  'commit', 'robot', 'code_off', 'php', 'rebase_edit',

  // Design & Creative
  'design_services', 'web_stories', 'markdown', 'animated_images', 'gif_box',
  'stacks', 'package_2', 'instant_mix',

  // Smart Home & IoT (Nest products)
  'nest_farsight_eco', 'nest_cam_wired_stand', 'nest_doorbell_visitor', 'nest_protect',
  'nest_thermostat', 'nest_wifi_router', 'nest_clock_farsight_analog',
  'nest_clock_farsight_digital', 'nest_remote', 'nest_true_radiant',
  'nest_wake_on_approach', 'nest_wake_on_press', 'nest_display', 'nest_eco_leaf',
  'nest_heat_link', 'nest_sunblock',

  // Counter icons
  'counter_1', 'counter_2', 'counter_3', 'counter_4', 'counter_5',
  'counter_6', 'counter_7', 'counter_8', 'counter_9',

  // Media & Content
  'movie_edit', 'audio_video_receiver', 'manga', 'sports_score',

  // UI & Navigation
  'splitscreen_vertical_add', 'splitscreen_left', 'splitscreen_right',
  'collapse_content', 'expand_content', 'keep', 'keep_off',

  // Communication
  'chat_add_on', 'forums', 'news', 'subscriptions',

  // Security & Auth
  'passkey',

  // Assistant & AI
  'assistant', 'assistant_on_hub', 'home_app_logo',

  // Modern additions (likely Symbols-only)
  'demography', 'conveyor_belt', 'forklift', 'front_loader', 'trolley',
  'propane', 'propane_tank', 'electric_meter', 'gas_meter', 'heat_pump',
  'oil_barrel', 'solar_power', 'wind_power',
]);

/**
 * Get the appropriate icon class for an icon name
 * @param {string} iconName - The material icon name
 * @returns {string} - The CSS class to use ('material-icons' or 'material-symbols-outlined')
 */
const getIconClass = (iconName) => {
  return materialSymbolsOnly.has(iconName) ? 'material-symbols-outlined' : 'material-icons';
};

// Curated list of ~1500 commonly used Material Icons & Symbols
// Excludes: excessive resolution indicators (5k-10k, 2mp-24mp), detailed signal bars, and niche variants
const allMaterialIcons = [
  // Basic & Numbers
  '123', '360', '3d_rotation', '4k', '6_ft_apart', '18_up_rating',

  // Connectivity & Mobile Data
  '1x_mobiledata', '3g_mobiledata', '3p', '4g_mobiledata', '4g_plus_mobiledata', '5g',
  'e_mobiledata', 'g_mobiledata', 'h_mobiledata', 'h_plus_mobiledata', 'lte_mobiledata',
  'lte_plus_mobiledata', 'r_mobiledata', 'mobiledata_off',

  // Basics & Common
  'abc', 'ac_unit', 'access_alarm', 'access_alarms', 'access_time', 'access_time_filled',
  'accessibility', 'accessibility_new', 'accessible', 'accessible_forward',

  // Account & Profile
  'account_balance', 'account_balance_wallet', 'account_box', 'account_circle',
  'account_tree', 'admin_panel_settings', 'manage_accounts', 'person', 'person_add',
  'person_add_alt', 'person_add_alt_1', 'person_add_disabled', 'person_off',
  'person_outline', 'person_pin', 'person_pin_circle', 'person_remove',
  'person_remove_alt_1', 'person_search', 'supervised_user_circle', 'supervisor_account',

  // Actions - Add
  'add', 'add_a_photo', 'add_alarm', 'add_alert', 'add_box', 'add_business', 'add_call',
  'add_card', 'add_chart', 'add_circle', 'add_circle_outline', 'add_comment', 'add_home',
  'add_home_work', 'add_ic_call', 'add_link', 'add_location', 'add_location_alt',
  'add_moderator', 'add_photo_alternate', 'add_reaction', 'add_road', 'add_shopping_cart',
  'add_task', 'add_to_drive', 'add_to_home_screen', 'add_to_photos', 'add_to_queue',
  'addchart',

  // Actions - Remove
  'remove', 'remove_circle', 'remove_circle_outline', 'remove_done', 'remove_from_queue',
  'remove_moderator', 'remove_red_eye', 'remove_road', 'remove_shopping_cart',

  // Actions - Basic
  'adjust', 'adb', 'archive', 'backspace', 'backup', 'backup_table', 'block',
  'block_flipped', 'build', 'build_circle', 'cached', 'cancel', 'cancel_presentation',
  'cancel_schedule_send', 'check', 'check_box', 'check_box_outline_blank', 'check_circle',
  'check_circle_outline', 'clear', 'clear_all', 'close', 'close_fullscreen', 'compress',
  'content_copy', 'content_cut', 'content_paste', 'content_paste_go', 'content_paste_off',
  'content_paste_search', 'copy_all', 'create', 'create_new_folder', 'delete',
  'delete_forever', 'delete_outline', 'delete_sweep', 'done', 'done_all', 'done_outline',
  'download', 'download_done', 'download_for_offline', 'downloading', 'drag_handle',
  'drag_indicator', 'edit', 'edit_attributes', 'edit_calendar', 'edit_document',
  'edit_location', 'edit_location_alt', 'edit_note', 'edit_notifications', 'edit_off',
  'edit_road', 'edit_square', 'expand', 'expand_circle_down', 'expand_less', 'expand_more',
  'file_copy', 'file_download', 'file_download_done', 'file_download_off', 'file_open',
  'file_present', 'file_upload', 'file_upload_off', 'filter_list', 'filter_list_alt',
  'filter_list_off', 'find_in_page', 'find_replace', 'flip', 'flip_to_back',
  'flip_to_front', 'get_app', 'grade', 'hide_image', 'hide_source', 'highlight',
  'highlight_alt', 'highlight_off', 'highlight_remove', 'history', 'history_edu',
  'history_toggle_off', 'home', 'home_filled', 'home_max', 'home_mini',
  'home_repair_service', 'home_work', 'inbox', 'info', 'info_outline', 'input',
  'inventory', 'inventory_2', 'join_full', 'join_inner', 'join_left', 'join_right',
  'key', 'key_off', 'label', 'label_important', 'label_important_outline', 'label_off',
  'label_outline', 'launch', 'link', 'link_off', 'list', 'list_alt', 'lock', 'lock_clock',
  'lock_open', 'lock_outline', 'lock_person', 'lock_reset', 'login', 'logout',
  'markunread', 'markunread_mailbox', 'maximize', 'menu', 'menu_book', 'menu_open',
  'merge', 'merge_type', 'minimize', 'more', 'more_horiz', 'more_time', 'more_vert',
  'move_down', 'move_to_inbox', 'move_up', 'open_in_browser', 'open_in_full',
  'open_in_new', 'open_in_new_off', 'open_with', 'output', 'padding', 'pan_tool',
  'pan_tool_alt', 'pin', 'pin_drop', 'pin_end', 'pin_invoke', 'pinch', 'push_pin',
  'redo', 'refresh', 'remember_me', 'reorder', 'reply', 'reply_all', 'report',
  'report_off', 'report_problem', 'restore', 'restore_from_trash', 'restore_page',
  'rotate_90_degrees_ccw', 'rotate_90_degrees_cw', 'rotate_left', 'rotate_right',
  'save', 'save_alt', 'save_as', 'saved_search', 'savings', 'schedule', 'schedule_send',
  'search', 'search_off', 'select_all', 'send', 'send_and_archive', 'send_time_extension',
  'send_to_mobile', 'settings', 'settings_accessibility', 'settings_applications',
  'settings_backup_restore', 'settings_bluetooth', 'settings_brightness', 'settings_cell',
  'settings_display', 'settings_ethernet', 'settings_input_antenna',
  'settings_input_component', 'settings_input_composite', 'settings_input_hdmi',
  'settings_input_svideo', 'settings_overscan', 'settings_phone', 'settings_power',
  'settings_remote', 'settings_suggest', 'settings_system_daydream', 'settings_voice',
  'share', 'share_arrival_time', 'share_location', 'shortcut', 'sort', 'sort_by_alpha',
  'source', 'star', 'star_border', 'star_half', 'star_outline', 'star_rate', 'stars',
  'start', 'straighten', 'style', 'subscript', 'superscript', 'swap_calls', 'swap_horiz',
  'swap_horizontal_circle', 'swap_vert', 'swap_vert_circle', 'swap_vertical_circle',
  'sync', 'sync_alt', 'sync_disabled', 'sync_lock', 'sync_problem', 'task', 'task_alt',
  'thumbs_up_down', 'toggle_off', 'toggle_on', 'transform', 'translate', 'troubleshoot',
  'tune', 'undo', 'unarchive', 'unfold_less', 'unfold_less_double', 'unfold_more',
  'unfold_more_double', 'unpublished', 'unsubscribe', 'update', 'update_disabled',
  'upgrade', 'upload', 'upload_file', 'verified', 'verified_user', 'view_in_ar',
  'visibility', 'visibility_off', 'watch_later', 'watch_off', 'wrap_text', 'zoom_in',
  'zoom_in_map', 'zoom_out', 'zoom_out_map',

  // Arrows & Navigation
  'arrow_back', 'arrow_back_ios', 'arrow_back_ios_new', 'arrow_circle_down',
  'arrow_circle_left', 'arrow_circle_right', 'arrow_circle_up', 'arrow_downward',
  'arrow_drop_down', 'arrow_drop_down_circle', 'arrow_drop_up', 'arrow_forward',
  'arrow_forward_ios', 'arrow_left', 'arrow_outward', 'arrow_right', 'arrow_right_alt',
  'arrow_upward', 'chevron_left', 'chevron_right', 'double_arrow', 'east', 'first_page',
  'keyboard_arrow_down', 'keyboard_arrow_left', 'keyboard_arrow_right', 'keyboard_arrow_up',
  'keyboard_double_arrow_down', 'keyboard_double_arrow_left', 'keyboard_double_arrow_right',
  'keyboard_double_arrow_up', 'last_page', 'navigate_before', 'navigate_next', 'navigation',
  'north', 'north_east', 'north_west', 'south', 'south_east', 'south_west',
  'subdirectory_arrow_left', 'subdirectory_arrow_right', 'turn_left', 'turn_right',
  'turn_sharp_left', 'turn_sharp_right', 'turn_slight_left', 'turn_slight_right',
  'u_turn_left', 'u_turn_right', 'west',

  // Alignment & Layout
  'align_horizontal_center', 'align_horizontal_left', 'align_horizontal_right',
  'align_vertical_bottom', 'align_vertical_center', 'align_vertical_top',
  'horizontal_distribute', 'horizontal_rule', 'horizontal_split', 'vertical_align_bottom',
  'vertical_align_center', 'vertical_align_top', 'vertical_distribute', 'vertical_split',

  // Media & Player Controls
  'album', 'audiotrack', 'av_timer', 'fast_forward', 'fast_rewind', 'forward_10',
  'forward_30', 'forward_5', 'loop', 'music_note', 'music_off', 'music_video', 'pause',
  'pause_circle', 'pause_circle_filled', 'pause_circle_outline', 'pause_presentation',
  'play_arrow', 'play_circle', 'play_circle_fill', 'play_circle_filled',
  'play_circle_outline', 'play_disabled', 'play_for_work', 'play_lesson', 'playlist_add',
  'playlist_add_check', 'playlist_add_check_circle', 'playlist_add_circle',
  'playlist_play', 'playlist_remove', 'queue', 'queue_music', 'queue_play_next',
  'repeat', 'repeat_on', 'repeat_one', 'repeat_one_on', 'replay', 'replay_10',
  'replay_30', 'replay_5', 'replay_circle_filled', 'shuffle', 'shuffle_on', 'skip_next',
  'skip_previous', 'slow_motion_video', 'snooze', 'speed', 'stop', 'stop_circle',
  'stop_screen_share', 'subscriptions', 'subtitles', 'subtitles_off', 'surround_sound',
  'video_call', 'video_camera_back', 'video_camera_front', 'video_chat',
  'video_collection', 'video_file', 'video_label', 'video_library', 'video_settings',
  'video_stable', 'videocam', 'videocam_off', 'volume_down', 'volume_down_alt',
  'volume_mute', 'volume_off', 'volume_up',

  // Images & Camera
  'add_a_photo', 'add_photo_alternate', 'add_to_photos', 'adjust', 'animation',
  'art_track', 'assistant_photo', 'auto_awesome', 'auto_awesome_mosaic',
  'auto_awesome_motion', 'auto_fix_high', 'auto_fix_normal', 'auto_fix_off', 'auto_stories',
  'blur_circular', 'blur_linear', 'blur_off', 'blur_on', 'brightness_auto',
  'brightness_high', 'brightness_low', 'brightness_medium', 'broken_image', 'brush',
  'burst_mode', 'camera', 'camera_alt', 'camera_enhance', 'camera_front', 'camera_indoor',
  'camera_outdoor', 'camera_rear', 'camera_roll', 'cameraswitch', 'center_focus_strong',
  'center_focus_weak', 'collections', 'collections_bookmark', 'color_lens', 'colorize',
  'compare', 'control_point', 'control_point_duplicate', 'crop', 'crop_16_9', 'crop_3_2',
  'crop_5_4', 'crop_7_5', 'crop_din', 'crop_free', 'crop_landscape', 'crop_original',
  'crop_portrait', 'crop_rotate', 'crop_square', 'deblur', 'dehaze', 'details', 'dirty_lens',
  'edit', 'euro', 'exposure', 'exposure_minus_1', 'exposure_minus_2', 'exposure_neg_1',
  'exposure_neg_2', 'exposure_plus_1', 'exposure_plus_2', 'exposure_zero',
  'face_retouching_natural', 'face_retouching_off', 'filter', 'filter_1', 'filter_2',
  'filter_3', 'filter_4', 'filter_5', 'filter_6', 'filter_7', 'filter_8', 'filter_9',
  'filter_9_plus', 'filter_alt', 'filter_alt_off', 'filter_b_and_w', 'filter_center_focus',
  'filter_drama', 'filter_frames', 'filter_hdr', 'filter_none', 'filter_tilt_shift',
  'filter_vintage', 'flare', 'flash_auto', 'flash_off', 'flash_on', 'flip_camera_android',
  'flip_camera_ios', 'gradient', 'grain', 'grid_off', 'grid_on', 'hdr_auto',
  'hdr_auto_select', 'hdr_enhanced_select', 'hdr_off', 'hdr_off_select', 'hdr_on',
  'hdr_on_select', 'hdr_plus', 'hdr_strong', 'hdr_weak', 'healing', 'hevc', 'hide_image',
  'image', 'image_aspect_ratio', 'image_not_supported', 'image_search',
  'imagesearch_roller', 'iso', 'leak_add', 'leak_remove', 'lens', 'lens_blur',
  'linked_camera', 'looks', 'looks_3', 'looks_4', 'looks_5', 'looks_6', 'looks_one',
  'looks_two', 'loupe', 'monochrome_photos', 'motion_photos_auto', 'motion_photos_off',
  'motion_photos_on', 'motion_photos_pause', 'motion_photos_paused', 'movie_creation',
  'movie_edit', 'movie_filter', 'music_off', 'nat', 'nature', 'nature_people',
  'navigate_before', 'navigate_next', 'palette', 'panorama', 'panorama_fish_eye',
  'panorama_fisheye', 'panorama_horizontal', 'panorama_horizontal_select',
  'panorama_photosphere', 'panorama_photosphere_select', 'panorama_vertical',
  'panorama_vertical_select', 'panorama_wide_angle', 'panorama_wide_angle_select',
  'party_mode', 'photo', 'photo_album', 'photo_camera', 'photo_camera_back',
  'photo_camera_front', 'photo_filter', 'photo_library', 'photo_size_select_actual',
  'photo_size_select_large', 'photo_size_select_small', 'picture_as_pdf',
  'picture_in_picture', 'picture_in_picture_alt', 'portrait', 'remove_red_eye', 'rotate_left',
  'rotate_right', 'shutter_speed', 'slideshow', 'straighten', 'style', 'switch_camera',
  'switch_video', 'tag_faces', 'texture', 'timelapse', 'timer', 'timer_10', 'timer_10_select',
  'timer_3', 'timer_3_select', 'timer_off', 'tonality', 'transform', 'tune', 'vignette',
  'wb_auto', 'wb_cloudy', 'wb_incandescent', 'wb_iridescent', 'wb_shade', 'wb_sunny',
  'wb_twilight',

  // Communication
  'add_comment', 'alternate_email', 'attach_email', 'call', 'call_end', 'call_made',
  'call_merge', 'call_missed', 'call_missed_outgoing', 'call_received', 'call_split',
  'cancel_presentation', 'cell_tower', 'cell_wifi', 'chat', 'chat_bubble',
  'chat_bubble_outline', 'comment', 'comment_bank', 'comments_disabled', 'contact_mail',
  'contact_page', 'contact_phone', 'contact_support', 'contacts', 'dialer_sip', 'dialpad',
  'domain_verification', 'drafts', 'duo', 'email', 'forum', 'forward_to_inbox',
  'import_contacts', 'invert_colors_off', 'live_help', 'mail', 'mail_lock', 'mail_outline',
  'mark_as_unread', 'mark_chat_read', 'mark_chat_unread', 'mark_email_read',
  'mark_email_unread', 'mark_unread_chat_alt', 'message', 'messenger', 'messenger_outline',
  'mobile_screen_share', 'mode_comment', 'nat', 'no_sim', 'outgoing_mail', 'phone',
  'phone_android', 'phone_bluetooth_speaker', 'phone_callback', 'phone_disabled',
  'phone_enabled', 'phone_forwarded', 'phone_in_talk', 'phone_iphone', 'phone_locked',
  'phone_missed', 'phone_paused', 'phonelink', 'phonelink_erase', 'phonelink_lock',
  'phonelink_off', 'phonelink_ring', 'phonelink_setup', 'portable_wifi_off',
  'present_to_all', 'print_disabled', 'qr_code', 'qr_code_2', 'qr_code_scanner',
  'quick_contacts_dialer', 'quick_contacts_mail', 'read_more', 'ring_volume', 'rss_feed',
  'rtt', 'screen_share', 'send', 'sentiment_satisfied_alt', 'speaker_notes',
  'speaker_notes_off', 'speaker_phone', 'stay_current_landscape', 'stay_current_portrait',
  'stay_primary_landscape', 'stay_primary_portrait', 'stop_screen_share', 'swap_calls',
  'textsms', 'unsubscribe', 'voicemail', 'vpn_key', 'vpn_key_off', 'vpn_lock',

  // Files & Folders
  'article', 'attachment', 'cloud', 'cloud_circle', 'cloud_done', 'cloud_download',
  'cloud_off', 'cloud_queue', 'cloud_sync', 'cloud_upload', 'create_new_folder',
  'description', 'drive_eta', 'drive_file_move', 'drive_file_move_outline',
  'drive_file_move_rtl', 'drive_file_rename_outline', 'drive_folder_upload', 'folder',
  'folder_copy', 'folder_delete', 'folder_off', 'folder_open', 'folder_shared',
  'folder_special', 'folder_zip', 'format_overline', 'insert_drive_file', 'note',
  'note_add', 'note_alt', 'request_page', 'rule_folder', 'snippet_folder', 'source',
  'text_snippet', 'topic', 'upload_file',

  // Text Formatting
  'add_comment', 'attach_file', 'border_all', 'border_bottom', 'border_clear',
  'border_color', 'border_horizontal', 'border_inner', 'border_left', 'border_outer',
  'border_right', 'border_style', 'border_top', 'border_vertical', 'format_align_center',
  'format_align_justify', 'format_align_left', 'format_align_right', 'format_bold',
  'format_clear', 'format_color_fill', 'format_color_reset', 'format_color_text',
  'format_indent_decrease', 'format_indent_increase', 'format_italic',
  'format_line_spacing', 'format_list_bulleted', 'format_list_bulleted_add',
  'format_list_numbered', 'format_list_numbered_rtl', 'format_overline', 'format_paint',
  'format_quote', 'format_shapes', 'format_size', 'format_strikethrough',
  'format_textdirection_l_to_r', 'format_textdirection_r_to_l', 'format_underline',
  'format_underlined', 'functions', 'highlight', 'insert_chart', 'insert_chart_outlined',
  'insert_comment', 'insert_emoticon', 'insert_link', 'insert_photo', 'mode_comment',
  'mode_edit', 'mode_edit_outline', 'notes', 'pie_chart', 'pie_chart_outline',
  'pie_chart_outlined', 'post_add', 'publish', 'short_text', 'space_bar', 'spellcheck',
  'strikethrough_s', 'subscript', 'superscript', 'text_decrease', 'text_fields',
  'text_format', 'text_increase', 'text_rotate_up', 'text_rotate_vertical',
  'text_rotation_angledown', 'text_rotation_angleup', 'text_rotation_down',
  'text_rotation_none', 'title', 'vertical_align_bottom', 'vertical_align_center',
  'vertical_align_top', 'wrap_text',

  // Business & Commerce
  'account_balance', 'account_balance_wallet', 'add_business', 'add_card',
  'add_shopping_cart', 'approval', 'assessment', 'attach_money', 'atm', 'badge',
  'balance', 'business', 'business_center', 'calculate', 'card_giftcard',
  'card_membership', 'card_travel', 'credit_card', 'credit_card_off', 'credit_score',
  'currency_bitcoin', 'currency_exchange', 'currency_franc', 'currency_lira',
  'currency_pound', 'currency_ruble', 'currency_rupee', 'currency_yen', 'currency_yuan',
  'diamond', 'discount', 'euro', 'euro_symbol', 'local_atm', 'local_offer', 'monetization_on',
  'money', 'money_off', 'money_off_csred', 'paid', 'payment', 'payments', 'point_of_sale',
  'price_change', 'price_check', 'receipt', 'receipt_long', 'redeem', 'request_quote',
  'savings', 'sell', 'shopping_bag', 'shopping_basket', 'shopping_cart',
  'shopping_cart_checkout', 'store', 'store_mall_directory', 'storefront', 'toll',
  'wallet', 'wallet_giftcard', 'wallet_membership', 'wallet_travel', 'work', 'work_history',
  'work_off', 'work_outline',

  // Devices & Hardware
  'adb', 'ad_units', 'airplanemode_off', 'airplanemode_on', 'battery_alert',
  'battery_charging_full', 'battery_full', 'battery_saver', 'battery_unknown', 'bluetooth',
  'bluetooth_audio', 'bluetooth_connected', 'bluetooth_disabled', 'bluetooth_drive',
  'bluetooth_searching', 'brightness_auto', 'brightness_high', 'brightness_low',
  'brightness_medium', 'cast', 'cast_connected', 'cast_for_education', 'computer',
  'connected_tv', 'desktop_mac', 'desktop_windows', 'developer_board',
  'developer_board_off', 'developer_mode', 'device_hub', 'device_thermostat',
  'device_unknown', 'devices', 'devices_fold', 'devices_other', 'dock', 'dvr',
  'gamepad', 'headphones', 'headphones_battery', 'headset', 'headset_mic', 'headset_off',
  'home_max', 'home_mini', 'keyboard', 'keyboard_alt', 'keyboard_hide', 'keyboard_voice',
  'laptop', 'laptop_chromebook', 'laptop_mac', 'laptop_windows', 'memory', 'mouse',
  'phone_android', 'phone_iphone', 'phonelink', 'phonelink_off', 'power_input',
  'power_settings_new', 'router', 'scanner', 'security', 'sim_card', 'sim_card_alert',
  'sim_card_download', 'smart_display', 'smart_screen', 'smart_toy', 'smartphone',
  'speaker', 'speaker_group', 'storage', 'tablet', 'tablet_android', 'tablet_mac', 'toys',
  'tv', 'tv_off', 'usb', 'usb_off', 'videogame_asset', 'videogame_asset_off', 'watch',
  'watch_later', 'watch_off',

  // Time & Calendar
  'access_alarm', 'access_alarms', 'access_time', 'access_time_filled', 'add_alarm',
  'alarm', 'alarm_add', 'alarm_off', 'alarm_on', 'calendar_month', 'calendar_today',
  'calendar_view_day', 'calendar_view_month', 'calendar_view_week', 'date_range', 'event',
  'event_available', 'event_busy', 'event_note', 'event_repeat', 'history',
  'history_toggle_off', 'hourglass_bottom', 'hourglass_disabled', 'hourglass_empty',
  'hourglass_full', 'hourglass_top', 'next_week', 'pending', 'pending_actions',
  'query_builder', 'schedule', 'schedule_send', 'timer', 'timer_10', 'timer_10_select',
  'timer_3', 'timer_3_select', 'timer_off', 'today', 'update', 'upcoming', 'watch_later',

  // Places & Location
  'add_location', 'add_location_alt', 'atm', 'attractions', 'beenhere', 'castle', 'church',
  'cottage', 'departure_board', 'directions', 'directions_bike', 'directions_boat',
  'directions_boat_filled', 'directions_bus', 'directions_bus_filled', 'directions_car',
  'directions_car_filled', 'directions_off', 'directions_railway', 'directions_railway_filled',
  'directions_run', 'directions_subway', 'directions_subway_filled', 'directions_train',
  'directions_transit', 'directions_transit_filled', 'directions_walk', 'edit_location',
  'edit_location_alt', 'edit_road', 'ev_station', 'factory', 'flight', 'flight_land',
  'flight_takeoff', 'fort', 'house', 'layers', 'layers_clear', 'local_activity',
  'local_airport', 'local_atm', 'local_attraction', 'local_bar', 'local_cafe',
  'local_car_wash', 'local_convenience_store', 'local_dining', 'local_drink',
  'local_fire_department', 'local_florist', 'local_gas_station', 'local_grocery_store',
  'local_hospital', 'local_hotel', 'local_laundry_service', 'local_library', 'local_mall',
  'local_movies', 'local_offer', 'local_parking', 'local_pharmacy', 'local_phone',
  'local_pizza', 'local_play', 'local_police', 'local_post_office', 'local_printshop',
  'local_restaurant', 'local_see', 'local_shipping', 'local_taxi', 'location_city',
  'location_disabled', 'location_off', 'location_on', 'location_pin', 'location_searching',
  'map', 'moped', 'mosque', 'my_location', 'navigation', 'near_me', 'near_me_disabled',
  'not_listed_location', 'park', 'pedal_bike', 'person_pin', 'person_pin_circle', 'place',
  'pin_drop', 'public', 'restaurant', 'restaurant_menu', 'route', 'run_circle', 'sailing',
  'satellite', 'satellite_alt', 'store', 'store_mall_directory', 'storefront', 'subway',
  'synagogue', 'taxi_alert', 'temple_buddhist', 'temple_hindu', 'terrain', 'traffic',
  'train', 'tram', 'transfer_within_a_station', 'transit_enterexit', 'trip_origin',
  'two_wheeler', 'warehouse', 'where_to_vote', 'wrong_location', 'zoom_out_map',

  // Social & People
  'boy', 'cake', 'child_care', 'child_friendly', 'diversity_1', 'diversity_2', 'diversity_3',
  'elderly', 'elderly_woman', 'emoji_emotions', 'emoji_events', 'emoji_flags',
  'emoji_food_beverage', 'emoji_nature', 'emoji_objects', 'emoji_people', 'emoji_symbols',
  'emoji_transportation', 'engineering', 'face', 'face_2', 'face_3', 'face_4', 'face_5',
  'face_6', 'female', 'girl', 'group', 'group_add', 'group_off', 'group_remove',
  'group_work', 'groups', 'groups_2', 'groups_3', 'man', 'man_2', 'man_3', 'man_4',
  'military_tech', 'mood', 'mood_bad', 'paragliding', 'people', 'people_alt',
  'people_outline', 'person', 'pregnant_woman', 'psychology', 'psychology_alt',
  'public', 'real_estate_agent', 'reduce_capacity', 'school', 'science', 'self_improvement',
  'sentiment_dissatisfied', 'sentiment_neutral', 'sentiment_satisfied',
  'sentiment_satisfied_alt', 'sentiment_very_dissatisfied', 'sentiment_very_satisfied',
  'sick', 'social_distance', 'sports', 'sports_bar', 'sports_baseball', 'sports_basketball',
  'sports_cricket', 'sports_esports', 'sports_football', 'sports_golf', 'sports_gymnastics',
  'sports_handball', 'sports_hockey', 'sports_kabaddi', 'sports_martial_arts', 'sports_mma',
  'sports_motorsports', 'sports_rugby', 'sports_score', 'sports_soccer', 'sports_tennis',
  'sports_volleyball', 'support', 'support_agent', 'volunteer_activism', 'waving_hand',
  'woman', 'woman_2',

  // Food & Dining
  'bakery_dining', 'bento', 'breakfast_dining', 'brunch_dining', 'cake', 'coffee',
  'coffee_maker', 'delivery_dining', 'dinner_dining', 'egg', 'egg_alt', 'fastfood',
  'flatware', 'food_bank', 'icecream', 'kebab_dining', 'liquor', 'local_bar', 'local_cafe',
  'local_dining', 'local_drink', 'local_pizza', 'lunch_dining', 'no_food', 'no_meals',
  'outdoor_grill', 'ramen_dining', 'restaurant', 'restaurant_menu', 'rice_bowl',
  'room_service', 'set_meal', 'soup_kitchen', 'takeout_dining', 'tapas', 'wine_bar',

  // Home & Living
  'apartment', 'architecture', 'balcony', 'bathtub', 'bed', 'bedroom_baby', 'bedroom_child',
  'bedroom_parent', 'blender', 'blinds', 'blinds_closed', 'cabin', 'chair', 'chair_alt',
  'chalet', 'checkroom', 'coffee_maker', 'cottage', 'countertops', 'crib', 'curtains',
  'curtains_closed', 'deck', 'desk', 'dining', 'door_back', 'door_front', 'door_sliding',
  'doorbell', 'elevator', 'escalator', 'escalator_warning', 'foundation', 'garage',
  'gite', 'holiday_village', 'house', 'house_siding', 'houseboat', 'hvac', 'iron',
  'kitchen', 'living', 'microwave', 'other_houses', 'roofing', 'room', 'room_preferences',
  'room_service', 'sensor_door', 'sensor_window', 'shelves', 'shower', 'single_bed',
  'stairs', 'table_bar', 'table_restaurant', 'umbrella', 'villa', 'wash', 'water_damage',
  'weekend', 'window', 'yard',

  // Weather & Nature
  'ac_unit', 'air', 'cloudy_snowing', 'cyclone', 'dew_point', 'energy_savings_leaf', 'filter_drama',
  'flood', 'foggy', 'forest', 'grass', 'landslide', 'nights_stay', 'park', 'severe_cold',
  'snowing', 'storm', 'sunny', 'sunny_snowing', 'thunderstorm', 'tornado', 'tsunami',
  'volcano', 'water', 'water_drop', 'waves', 'wb_cloudy', 'wb_sunny', 'wb_twilight',

  // Transportation
  'airline_seat_flat', 'airline_seat_flat_angled', 'airline_seat_individual_suite',
  'airline_seat_legroom_extra', 'airline_seat_legroom_normal',
  'airline_seat_legroom_reduced', 'airline_seat_recline_extra',
  'airline_seat_recline_normal', 'airlines', 'airline_stops', 'airplane_ticket', 'airport_shuttle',
  'alt_route', 'bike_scooter', 'car_crash', 'car_rental', 'car_repair', 'commute',
  'connecting_airports', 'departure_board', 'directions_bike', 'directions_boat',
  'directions_boat_filled', 'directions_bus', 'directions_bus_filled', 'directions_car',
  'directions_car_filled', 'directions_railway', 'directions_railway_filled',
  'directions_run', 'directions_subway', 'directions_subway_filled', 'directions_train',
  'directions_transit', 'directions_transit_filled', 'directions_walk', 'drive_eta',
  'electric_bike', 'electric_car', 'electric_moped', 'electric_rickshaw',
  'electric_scooter', 'ev_station', 'flight', 'flight_class', 'flight_land',
  'flight_takeoff', 'fork_left', 'fork_right', 'local_airport', 'local_car_wash',
  'local_gas_station', 'local_parking', 'local_shipping', 'local_taxi', 'minor_crash',
  'moped', 'motorcycle', 'navigation', 'near_me', 'no_crash', 'pedal_bike', 'railway_alert',
  'ramp_left', 'ramp_right', 'roundabout_left', 'roundabout_right', 'route', 'rv_hookup',
  'subway', 'taxi_alert', 'train', 'tram', 'transfer_within_a_station', 'transit_enterexit',
  'trip_origin', 'two_wheeler',

  // Sports & Activities
  'downhill_skiing', 'hiking', 'ice_skating', 'kayaking', 'kitesurfing', 'nordic_walking',
  'paragliding', 'roller_skating', 'run_circle', 'running_with_errors', 'sailing',
  'scuba_diving', 'skateboarding', 'sledding', 'snowboarding', 'snowmobile', 'snowshoeing',
  'sports', 'sports_bar', 'sports_baseball', 'sports_basketball', 'sports_cricket',
  'sports_esports', 'sports_football', 'sports_golf', 'sports_gymnastics', 'sports_handball',
  'sports_hockey', 'sports_kabaddi', 'sports_martial_arts', 'sports_mma',
  'sports_motorsports', 'sports_rugby', 'sports_score', 'sports_soccer', 'sports_tennis',
  'sports_volleyball', 'surfing',

  // Health & Medical
  'add_moderator', 'airport_shuttle', 'baby_changing_station', 'back_hand', 'biotech',
  'bloodtype', 'clean_hands', 'cleaning_services', 'coronavirus', 'elderly', 'elderly_woman',
  'emergency', 'emergency_recording', 'emergency_share', 'face_retouching_natural',
  'family_restroom', 'favorite', 'favorite_border', 'favorite_outline', 'fitbit',
  'fitness_center', 'healing', 'health_and_safety', 'hearing', 'hearing_disabled',
  'heart_broken', 'local_hospital', 'local_pharmacy', 'masks', 'medical_information',
  'medical_services', 'medication', 'medication_liquid', 'monitor_heart', 'monitor_weight',
  'personal_injury', 'pregnant_woman', 'psychology', 'psychology_alt', 'sanitizer',
  'science', 'sensors', 'sensors_off', 'sick', 'smoke_free', 'smoking_rooms',
  'spa', 'vaccines', 'vape_free', 'vaping_rooms', 'volunteer_activism',

  // Technology & Development
  'adb', 'api', 'barcode_reader', 'bug_report', 'code', 'code_off', 'computer',
  'connected_tv', 'construction', 'css', 'data_array', 'data_object', 'dataset',
  'dataset_linked', 'desktop_mac', 'desktop_windows', 'developer_board',
  'developer_board_off', 'developer_mode', 'devices', 'devices_fold', 'devices_other',
  'dns', 'extension', 'extension_off', 'handyman', 'hardware', 'home_repair_service',
  'html', 'hub', 'integration_instructions', 'javascript', 'keyboard', 'keyboard_alt',
  'laptop', 'laptop_chromebook', 'laptop_mac', 'laptop_windows', 'memory', 'miscellaneous_services',
  'mouse', 'network_check', 'network_ping', 'pest_control', 'pest_control_rodent', 'php',
  'plumbing', 'power', 'power_off', 'precision_manufacturing', 'print', 'print_disabled',
  'qr_code', 'qr_code_2', 'qr_code_scanner', 'router', 'rss_feed', 'rtt', 'scanner',
  'schema', 'screenshot', 'screenshot_monitor', 'sdk', 'security', 'security_update',
  'security_update_good', 'security_update_warning', 'settings', 'settings_applications',
  'settings_bluetooth', 'settings_cell', 'settings_ethernet', 'settings_input_antenna',
  'settings_input_component', 'settings_input_composite', 'settings_input_hdmi',
  'settings_input_svideo', 'settings_power', 'settings_remote', 'sim_card', 'sim_card_alert',
  'sim_card_download', 'smartphone', 'storage', 'sync', 'sync_alt', 'sync_disabled',
  'sync_lock', 'sync_problem', 'system_security_update', 'system_security_update_good',
  'system_security_update_warning', 'system_update', 'system_update_alt', 'tablet',
  'tablet_android', 'tablet_mac', 'terminal', 'usb', 'usb_off', 'vpn_key', 'vpn_key_off',
  'vpn_lock', 'webhook', 'wifi', 'wifi_calling', 'wifi_calling_3', 'wifi_channel',
  'wifi_find', 'wifi_lock', 'wifi_off', 'wifi_password', 'wifi_protected_setup',
  'wifi_tethering', 'wifi_tethering_error', 'wifi_tethering_off',

  // Charts & Data Visualization
  'analytics', 'area_chart', 'assessment', 'bar_chart', 'bubble_chart', 'candlestick_chart',
  'data_exploration', 'data_thresholding', 'data_usage', 'donut_large', 'donut_small',
  'leaderboard', 'line_axis', 'multiline_chart', 'pie_chart', 'pie_chart_outline',
  'pie_chart_outlined', 'poll', 'scatter_plot', 'score', 'scoreboard', 'show_chart',
  'ssid_chart', 'stacked_bar_chart', 'stacked_line_chart', 'summarize', 'table_chart',
  'table_rows', 'table_view', 'timeline', 'trending_down', 'trending_flat', 'trending_neutral',
  'trending_up', 'waterfall_chart',

  // Views & Display
  'apps', 'dashboard', 'dashboard_customize', 'fullscreen', 'fullscreen_exit', 'grid_3x3',
  'grid_4x4', 'grid_goldenratio', 'grid_off', 'grid_on', 'grid_view', 'space_dashboard',
  'splitscreen', 'tab', 'tab_unselected', 'view_agenda', 'view_array', 'view_carousel',
  'view_column', 'view_comfortable', 'view_comfy', 'view_comfy_alt', 'view_compact',
  'view_compact_alt', 'view_cozy', 'view_day', 'view_headline', 'view_in_ar',
  'view_kanban', 'view_list', 'view_module', 'view_quilt', 'view_sidebar', 'view_stream',
  'view_timeline', 'view_week', 'web_asset', 'web_asset_off', 'widgets', 'window',

  // Alerts & Notifications
  'add_alert', 'campaign', 'circle_notifications', 'crisis_alert', 'dangerous', 'error',
  'error_outline', 'info', 'info_outline', 'new_releases', 'notification_add',
  'notification_important', 'notifications', 'notifications_active', 'notifications_none',
  'notifications_off', 'notifications_on', 'notifications_paused', 'priority_high',
  'privacy_tip', 'report', 'report_gmailerrorred', 'report_off', 'report_problem',
  'verified', 'verified_user', 'warning', 'warning_amber',

  // Brand & Social Media
  'adobe', 'apple', 'discord', 'facebook', 'fitbit', 'logo_dev', 'paypal', 'pix', 'quora',
  'reddit', 'shopify', 'snapchat', 'telegram', 'tiktok', 'wechat', 'woo_commerce',
  'wordpress', 'youtube_searched_for',

  // Miscellaneous
  'abc', 'animation', 'api', 'auto_graph', 'auto_mode', 'backpack', 'badge', 'block',
  'block_flipped', 'bookmark', 'bookmark_add', 'bookmark_added', 'bookmark_border',
  'bookmark_outline', 'bookmark_remove', 'bookmarks', 'brightness_1', 'brightness_2',
  'brightness_3', 'brightness_4', 'brightness_5', 'brightness_6', 'brightness_7',
  'casino', 'catching_pokemon', 'celebration', 'checklist', 'checklist_rtl', 'circle',
  'class', 'construction', 'copyright', 'cruelty_free', 'dark_mode', 'deselect',
  'difference', 'disabled_by_default', 'disabled_visible', 'do_not_disturb',
  'do_not_disturb_alt', 'do_not_disturb_off', 'do_not_disturb_on',
  'do_not_disturb_on_total_silence', 'do_not_step', 'do_not_touch', 'domain', 'domain_add',
  'domain_disabled', 'domain_verification', 'drag_indicator', 'event', 'event_available',
  'event_busy', 'event_note', 'event_repeat', 'event_seat', 'explicit', 'explore',
  'explore_off', 'extension', 'extension_off', 'festival', 'fingerprint', 'flag',
  'flag_circle', 'flaky', 'flutter_dash', 'gavel', 'generating_tokens', 'gesture',
  'gif', 'gif_box', 'goat', 'handshake', 'help', 'help_center', 'help_outline', 'hexagon',
  'interests', 'interpreter_mode', 'language', 'lightbulb', 'lightbulb_circle',
  'lightbulb_outline', 'mode', 'new_label', 'not_accessible', 'not_interested',
  'not_started', 'offline_bolt', 'offline_pin', 'offline_share', 'on_device_training',
  'opacity', 'outlet', 'outlined_flag', 'pages', 'pageview', 'pallet', 'pattern',
  'pentagon', 'percent', 'phishing', 'pinch', 'pivot_table_chart', 'plagiarism',
  'policy', 'polyline', 'polymer', 'preview', 'published_with_changes', 'puzzle',
  'quickreply', 'quiz', 'recycling', 'reviews', 'rocket', 'rocket_launch', 'rule',
  'safety_check', 'safety_divider', 'scale', 'screen_lock_landscape',
  'screen_lock_portrait', 'screen_lock_rotation', 'screen_rotation', 'screen_rotation_alt',
  'screen_search_desktop', 'segment', 'settings_suggest', 'shield', 'shield_moon',
  'sign_language', 'square', 'square_foot', 'stadium', 'sticky_note_2', 'swipe',
  'swipe_down', 'swipe_down_alt', 'swipe_left', 'swipe_left_alt', 'swipe_right',
  'swipe_right_alt', 'swipe_up', 'swipe_up_alt', 'swipe_vertical', 'switch_access_shortcut',
  'switch_access_shortcut_add', 'switch_account', 'switch_left', 'switch_right',
  'tag', 'theater_comedy', 'theaters', 'thermostat', 'thermostat_auto', 'tips_and_updates',
  'token', 'touch_app', 'tour', 'track_changes', 'transcribe', 'travel_explore',
  'type_specimen', 'upcoming', 'verified', 'verified_user', 'wysiwyg',
];

// For backward compatibility, keep separate arrays
// These are now just filtered views of the main allMaterialIcons array
const materialIcons = allMaterialIcons.filter(icon => !materialSymbolsOnly.has(icon));
const materialSymbols = allMaterialIcons.filter(icon => materialSymbolsOnly.has(icon));

export {
  materialSymbolsOnly,
  getIconClass,
  materialIcons,
  materialSymbols,
  allMaterialIcons,
};
