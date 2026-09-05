cmake_minimum_required(VERSION 3.22)
project(WhatsAppGo VERSION 0.1.0 LANGUAGES CXX)

set(CMAKE_CXX_STANDARD 20)
set(CMAKE_CXX_STANDARD_REQUIRED ON)
set(CMAKE_AUTOMOC ON)
set(CMAKE_AUTORCC ON)

find_package(Qt6 6.5 REQUIRED COMPONENTS Core Gui Widgets Quick QuickControls2 Network Multimedia)

qt_standard_project_setup(REQUIRES 6.5)
qt_policy(SET QTP0004 NEW)

qt_add_executable(whatsappgo
    src/main.cpp
    src/messagemodel.cpp
    src/messagemodel.h
    src/profilemonitor.cpp
    src/profilemonitor.h
    src/rpcclient.cpp
    src/rpcclient.h
    src/traybehavior.cpp
    src/traybehavior.h
)

qt_add_resources(whatsappgo "desktop-assets"
    PREFIX "/"
    BASE "${CMAKE_CURRENT_SOURCE_DIR}/../assets"
    FILES "${CMAKE_CURRENT_SOURCE_DIR}/../assets/org.whatsappgo.Desktop.svg"
)

# WhatsApp Web renders in Roboto, which it ships as a webfont so it looks the
# same on a machine that has never installed it. Qt can only use families the
# font database knows, so the interface carries its own copy instead of relying
# on a distribution package. Apache-2.0; see fonts/Roboto-COPYRIGHT.
#
# BIG_RESOURCES is required, not an optimisation. Without it `rcc` emits the
# fonts as a C++ byte array — roughly 1.5 million lines for the CJK collection
# alone — and the compiler needs gigabytes of scratch space to assemble it.
qt_add_resources(whatsappgo "bundled-fonts"
    BIG_RESOURCES
    PREFIX "/fonts"
    BASE "${CMAKE_CURRENT_SOURCE_DIR}/fonts"
    FILES
        "${CMAKE_CURRENT_SOURCE_DIR}/fonts/NotoSansArabic-Regular.ttf"
        "${CMAKE_CURRENT_SOURCE_DIR}/fonts/NotoSansArmenian-Regular.ttf"
        "${CMAKE_CURRENT_SOURCE_DIR}/fonts/NotoSansBengali-Regular.ttf"
        "${CMAKE_CURRENT_SOURCE_DIR}/fonts/NotoSansCJK-Regular.ttc"
        "${CMAKE_CURRENT_SOURCE_DIR}/fonts/NotoSansCherokee-Regular.ttf"
        "${CMAKE_CURRENT_SOURCE_DIR}/fonts/NotoSansDevanagari-Regular.ttf"
        "${CMAKE_CURRENT_SOURCE_DIR}/fonts/NotoSansEthiopic-Regular.ttf"
        "${CMAKE_CURRENT_SOURCE_DIR}/fonts/NotoSansGeorgian-Regular.ttf"
        "${CMAKE_CURRENT_SOURCE_DIR}/fonts/NotoSansGujarati-Regular.ttf"
        "${CMAKE_CURRENT_SOURCE_DIR}/fonts/NotoSansGurmukhi-Regular.ttf"
        "${CMAKE_CURRENT_SOURCE_DIR}/fonts/NotoSansHebrew-Regular.ttf"
        "${CMAKE_CURRENT_SOURCE_DIR}/fonts/NotoSansKannada-Regular.ttf"
        "${CMAKE_CURRENT_SOURCE_DIR}/fonts/NotoSansKhmer-Regular.ttf"
        "${CMAKE_CURRENT_SOURCE_DIR}/fonts/NotoSansLao-Regular.ttf"
        "${CMAKE_CURRENT_SOURCE_DIR}/fonts/NotoSansMalayalam-Regular.ttf"
        "${CMAKE_CURRENT_SOURCE_DIR}/fonts/NotoSansMongolian-Regular.ttf"
        "${CMAKE_CURRENT_SOURCE_DIR}/fonts/NotoSansMyanmar-Regular.ttf"
        "${CMAKE_CURRENT_SOURCE_DIR}/fonts/NotoSansOriya-Regular.ttf"
        "${CMAKE_CURRENT_SOURCE_DIR}/fonts/NotoSansSinhala-Regular.ttf"
        "${CMAKE_CURRENT_SOURCE_DIR}/fonts/NotoSansSyriac-Regular.ttf"
        "${CMAKE_CURRENT_SOURCE_DIR}/fonts/NotoSansTamil-Regular.ttf"
        "${CMAKE_CURRENT_SOURCE_DIR}/fonts/NotoSansTelugu-Regular.ttf"
        "${CMAKE_CURRENT_SOURCE_DIR}/fonts/NotoSansThaana-Regular.ttf"
        "${CMAKE_CURRENT_SOURCE_DIR}/fonts/NotoSansThai-Regular.ttf"
        "${CMAKE_CURRENT_SOURCE_DIR}/fonts/Roboto-Bold.ttf"
        "${CMAKE_CURRENT_SOURCE_DIR}/fonts/Roboto-Medium.ttf"
        "${CMAKE_CURRENT_SOURCE_DIR}/fonts/Roboto-Regular.ttf"
)

set_source_files_properties(qml/Theme.qml PROPERTIES QT_QML_SINGLETON_TYPE TRUE)
set_source_files_properties(qml/Playback.qml PROPERTIES QT_QML_SINGLETON_TYPE TRUE)
set_source_files_properties(qml/SearchHighlight.qml PROPERTIES QT_QML_SINGLETON_TYPE TRUE)
set_source_files_properties(qml/RowTime.qml PROPERTIES QT_QML_SINGLETON_TYPE TRUE)

qt_add_qml_module(whatsappgo
    URI org.whatsappgo
    VERSION 1.0
    NO_IMPORT_SCAN
    QML_FILES
        qml/Main.qml
        qml/Theme.qml
        qml/Playback.qml
        qml/SearchHighlight.qml
        qml/RowTime.qml
        qml/SearchResultsPane.qml
        qml/ChatSearchPanel.qml
        qml/PairingPage.qml
        qml/ChatListDelegate.qml
        qml/ChatFilterBar.qml
        qml/ChatFilterChip.qml
        qml/MediaLibraryPane.qml
        qml/AccountSwitcherButton.qml
        qml/StatusAvatar.qml
        qml/StatusPage.qml
        qml/StatusViewer.qml
        qml/ComposerLinkPreview.qml
        qml/NewChatPane.qml
        qml/MessageDelegate.qml
        qml/SelectableMessageText.qml
        qml/FeatureSection.qml
        qml/Avatar.qml
        qml/TintedIcon.qml
        qml/ThemedToolButton.qml
        qml/OverlayScrollBar.qml
        qml/ReadReceipt.qml
        qml/EmojiPicker.qml
        qml/AttachmentMenu.qml
        qml/SettingsPane.qml
        qml/SettingsRow.qml
        qml/SettingsToggleRow.qml
        qml/SettingsChoiceRow.qml
        qml/DialogRadioButton.qml
        qml/DialogTextField.qml
        qml/WhatsAppDialog.qml
        qml/WhatsAppMenuPopup.qml
        qml/WhatsAppMenuItem.qml
        qml/VoiceRecorder.qml
        qml/MediaPreview.qml
        qml/ChatMediaViewer.qml
        qml/ContactInfoDrawer.qml
		qml/MessageInfoDrawer.qml
    RESOURCES
        qml/icons/new-chat.svg
        qml/icons/menu.svg
        qml/icons/search.svg
        qml/icons/attach.svg
        qml/icons/send.svg
        qml/icons/mic.svg
        qml/icons/stop.svg
        qml/icons/close.svg
        qml/icons/smile.svg
        qml/icons/reconnect.svg
        qml/icons/logout.svg
        qml/icons/reply.svg
        qml/icons/heart.svg
        qml/icons/edit.svg
        qml/icons/delete.svg
        qml/icons/copy.svg
        qml/icons/rotate-left.svg
        qml/icons/rotate-right.svg
        qml/icons/user.svg
        qml/icons/pin.svg
        qml/icons/archive.svg
        qml/icons/back.svg
        qml/icons/mute.svg
        qml/icons/chats.svg
        qml/icons/sun.svg
        qml/icons/moon.svg
        qml/icons/phone.svg
        qml/icons/video.svg
        qml/icons/gallery.svg
        qml/icons/document.svg
        qml/icons/camera.svg
        qml/icons/headphones.svg
        qml/icons/contact.svg
        qml/icons/poll.svg
        qml/icons/calendar.svg
        qml/icons/sticker.svg
        qml/icons/link.svg
		qml/icons/info.svg
        qml/icons/lock.svg
        qml/icons/chevron-right.svg
        qml/icons/plus.svg
        qml/assets/chat-background.png
)

target_link_libraries(whatsappgo PRIVATE
    Qt6::Core Qt6::Gui Qt6::Widgets Qt6::Quick Qt6::QuickControls2 Qt6::Network Qt6::Multimedia
)

target_compile_definitions(whatsappgo PRIVATE
    QT_NO_CAST_FROM_ASCII
    QT_NO_CAST_TO_ASCII
)

# The daemon carries the platform's executable suffix; the desktop client looks
# for it next to itself under exactly this name.
if(WIN32)
    set(WHATSAPPGO_DAEMON_SUFFIX ".exe")
else()
    set(WHATSAPPGO_DAEMON_SUFFIX "")
endif()

# A Windows GUI subsystem binary has no console, so anything it writes to
# stdout or stderr is discarded - including the reason a --*-test run failed,
# which makes a red CI job unreadable. The release scripts configure with
# BUILD_TESTING=OFF and get the GUI subsystem; a testing tree keeps its console.
if(BUILD_TESTING)
    set(WHATSAPPGO_WIN32_EXECUTABLE OFF)
else()
    set(WHATSAPPGO_WIN32_EXECUTABLE ON)
endif()

set_target_properties(whatsappgo PROPERTIES
    WIN32_EXECUTABLE ${WHATSAPPGO_WIN32_EXECUTABLE}
    # A .app bundle on macOS: the platform will not show a notification, a dock
    # icon or a menu bar for a bare executable.
    MACOSX_BUNDLE ON
    MACOSX_BUNDLE_BUNDLE_NAME "WhatsAppGo"
    MACOSX_BUNDLE_GUI_IDENTIFIER "org.whatsappgo.Desktop"
    MACOSX_BUNDLE_BUNDLE_VERSION "${PROJECT_VERSION}"
    MACOSX_BUNDLE_SHORT_VERSION_STRING "${PROJECT_VERSION}"
)

if(APPLE)
    # Inside the bundle the daemon sits beside the client in Contents/MacOS,
    # which is the first place daemonExecutable() looks.
    install(TARGETS whatsappgo BUNDLE DESTINATION .)
    install(PROGRAMS
        ${CMAKE_CURRENT_SOURCE_DIR}/../bin/whatsappd
        ${CMAKE_CURRENT_SOURCE_DIR}/../bin/whatsappctl
        DESTINATION whatsappgo.app/Contents/MacOS
    )
elseif(WIN32)
    install(TARGETS whatsappgo RUNTIME DESTINATION .)
    install(PROGRAMS
        ${CMAKE_CURRENT_SOURCE_DIR}/../bin/whatsappd${WHATSAPPGO_DAEMON_SUFFIX}
        ${CMAKE_CURRENT_SOURCE_DIR}/../bin/whatsappctl${WHATSAPPGO_DAEMON_SUFFIX}
        DESTINATION .
    )
else()
    install(TARGETS whatsappgo RUNTIME DESTINATION bin)
    install(PROGRAMS
        ${CMAKE_CURRENT_SOURCE_DIR}/../bin/whatsappd
        ${CMAKE_CURRENT_SOURCE_DIR}/../bin/whatsappctl
        DESTINATION bin
    )
    # Freedesktop metadata has no meaning on Windows or macOS, which describe
    # an application through the executable itself.
    install(FILES ../packaging/applications/org.whatsappgo.Desktop.desktop
            DESTINATION share/applications)
    install(FILES ../assets/org.whatsappgo.Desktop.svg
            DESTINATION share/icons/hicolor/scalable/apps)
endif()

include(CTest)
if(BUILD_TESTING)
    qt_add_executable(profile-monitor-test
        tests/ProfileMonitor.cpp
        src/profilemonitor.cpp
        src/profilemonitor.h
    )
    target_include_directories(profile-monitor-test PRIVATE src)
    target_link_libraries(profile-monitor-test PRIVATE Qt6::Core Qt6::Network)
    add_test(NAME desktop-profile-monitor COMMAND profile-monitor-test)
    set_tests_properties(desktop-profile-monitor PROPERTIES TIMEOUT 10)

    qt_add_executable(pairing-auto-start-test
        tests/PairingAutoStart.cpp
        src/messagemodel.cpp
        src/messagemodel.h
        src/profilemonitor.cpp
        src/profilemonitor.h
        src/rpcclient.cpp
        src/rpcclient.h
    )
    target_include_directories(pairing-auto-start-test PRIVATE src)
    target_link_libraries(pairing-auto-start-test PRIVATE Qt6::Core Qt6::Gui Qt6::Network)

    qt_add_executable(presence-updates-test
        tests/PresenceUpdates.cpp
        src/messagemodel.cpp
        src/messagemodel.h
        src/profilemonitor.cpp
        src/profilemonitor.h
        src/rpcclient.cpp
        src/rpcclient.h
    )
    target_include_directories(presence-updates-test PRIVATE src)
    target_link_libraries(presence-updates-test PRIVATE Qt6::Core Qt6::Gui Qt6::Network)

    qt_add_executable(chat-list-updates-test
        tests/ChatListUpdates.cpp
        src/messagemodel.cpp
        src/messagemodel.h
        src/profilemonitor.cpp
        src/profilemonitor.h
        src/rpcclient.cpp
        src/rpcclient.h
    )
    target_include_directories(chat-list-updates-test PRIVATE src)
    target_link_libraries(chat-list-updates-test PRIVATE Qt6::Core Qt6::Gui Qt6::Network)

    qt_add_executable(notification-routing-test
        tests/NotificationRouting.cpp
        src/messagemodel.cpp
        src/messagemodel.h
        src/profilemonitor.cpp
        src/profilemonitor.h
        src/rpcclient.cpp
        src/rpcclient.h
    )
    target_include_directories(notification-routing-test PRIVATE src)
    target_link_libraries(notification-routing-test PRIVATE Qt6::Core Qt6::Gui Qt6::Network)

    qt_add_executable(backend-probe-test
        tests/BackendProbe.cpp
        src/messagemodel.cpp
        src/messagemodel.h
        src/profilemonitor.cpp
        src/profilemonitor.h
        src/rpcclient.cpp
        src/rpcclient.h
    )
    target_include_directories(backend-probe-test PRIVATE src)
    target_link_libraries(backend-probe-test PRIVATE Qt6::Core Qt6::Gui Qt6::Network)

    qt_add_executable(link-preview-upgrade-test
        tests/LinkPreviewUpgrade.cpp
        src/messagemodel.cpp
        src/messagemodel.h
        src/profilemonitor.cpp
        src/profilemonitor.h
        src/rpcclient.cpp
        src/rpcclient.h
    )
    target_include_directories(link-preview-upgrade-test PRIVATE src)
    target_link_libraries(link-preview-upgrade-test PRIVATE Qt6::Core Qt6::Gui Qt6::Network)

    qt_add_executable(profile-display-names-test
        tests/ProfileDisplayNames.cpp
        src/messagemodel.cpp
        src/messagemodel.h
        src/profilemonitor.cpp
        src/profilemonitor.h
        src/rpcclient.cpp
        src/rpcclient.h
    )
    target_include_directories(profile-display-names-test PRIVATE src)
    target_link_libraries(profile-display-names-test PRIVATE Qt6::Core Qt6::Gui Qt6::Network)
    add_test(NAME desktop-profile-display-names COMMAND profile-display-names-test)
    add_test(NAME desktop-notification-routing COMMAND notification-routing-test)
    add_test(NAME desktop-backend-probe COMMAND backend-probe-test)
    set_tests_properties(desktop-profile-display-names PROPERTIES TIMEOUT 5)

    qt_add_executable(message-model-test
        tests/MessageModel.cpp
        src/messagemodel.cpp
        src/messagemodel.h
    )
    target_include_directories(message-model-test PRIVATE src)
    target_link_libraries(message-model-test PRIVATE Qt6::Core Qt6::Gui)
    add_test(NAME desktop-message-model COMMAND message-model-test)
    set_tests_properties(desktop-message-model PROPERTIES TIMEOUT 10)

    qt_add_executable(tray-behavior-test
        tests/TrayBehavior.cpp
        src/traybehavior.cpp
        src/traybehavior.h
    )
    target_include_directories(tray-behavior-test PRIVATE src)
    target_link_libraries(tray-behavior-test PRIVATE Qt6::Gui)
    add_test(NAME desktop-tray-behavior COMMAND tray-behavior-test)
    set_tests_properties(desktop-tray-behavior PROPERTIES TIMEOUT 10)

    add_test(NAME desktop-bundled-font COMMAND whatsappgo --bundled-font-test)
    set_tests_properties(desktop-bundled-font PROPERTIES
        ENVIRONMENT "QT_QPA_PLATFORM=offscreen"
        TIMEOUT 15
    )

    add_test(NAME desktop-qml-smoke COMMAND whatsappgo --smoke-test)
    set_tests_properties(desktop-qml-smoke PROPERTIES
        ENVIRONMENT "QT_QPA_PLATFORM=offscreen"
        TIMEOUT 10
    )
    add_test(NAME desktop-clean-startup
        COMMAND ${CMAKE_COMMAND}
            -DAPP=$<TARGET_FILE:whatsappgo>
            -P ${CMAKE_CURRENT_SOURCE_DIR}/tests/CleanStartup.cmake
    )
    set_tests_properties(desktop-clean-startup PROPERTIES TIMEOUT 10)
    add_test(NAME desktop-light-theme COMMAND whatsappgo --theme light --smoke-test)
    add_test(NAME desktop-dark-theme COMMAND whatsappgo --theme dark --smoke-test)
    add_test(NAME desktop-search-navigation COMMAND whatsappgo --search-navigation-test)
    add_test(NAME desktop-message-interactions COMMAND whatsappgo --message-interaction-test)
    add_test(NAME desktop-clipboard-images COMMAND whatsappgo --clipboard-image-test)
    add_test(NAME desktop-layout-regressions COMMAND whatsappgo --layout-regression-test)
    add_test(NAME desktop-media-preview COMMAND whatsappgo --media-preview-test)
    add_test(NAME desktop-chat-filters COMMAND whatsappgo --chat-filter-test)
    add_test(NAME desktop-chat-row-menu COMMAND whatsappgo --chat-row-menu-test)
    add_test(NAME desktop-search-results COMMAND whatsappgo --search-results-test)
    add_test(NAME desktop-profile-removal COMMAND whatsappgo --profile-removal-test)
    # The same removal with the per-account monitors constructed, which is how
    # the application runs it. WHATSAPPGO_BACKEND points at a program that exits
    # at once, so no daemon is left behind - which also means the monitors never
    # reach a live socket. This covers the teardown path; it does not reproduce
    # the crash that needed a connected monitor. See docs for that repro.
    add_test(NAME desktop-profile-removal-monitored COMMAND whatsappgo --profile-removal-test)
    add_test(NAME desktop-backend-lifecycle COMMAND whatsappgo --profile lifecycle-test --backend-lifecycle-test)
    add_test(NAME desktop-resize-rendering COMMAND whatsappgo --resize-rendering-test)
    add_test(NAME desktop-message-layout COMMAND whatsappgo --message-layout-test)
    add_test(NAME desktop-message-scroll COMMAND whatsappgo --message-scroll-test)
    add_test(NAME desktop-pairing-auto-start COMMAND pairing-auto-start-test)
    add_test(NAME desktop-presence-updates COMMAND presence-updates-test)
    add_test(NAME desktop-chat-list-updates COMMAND chat-list-updates-test)
    add_test(NAME desktop-link-preview-upgrade COMMAND link-preview-upgrade-test)
    add_test(NAME desktop-system-integration COMMAND whatsappgo --desktop-integration-test)
    add_test(NAME desktop-contact-info COMMAND whatsappgo --contact-info-test)
    add_test(NAME desktop-file-drop COMMAND whatsappgo --file-drop-test)
    add_test(NAME desktop-status-stories COMMAND whatsappgo --status-stories-test)
    add_test(NAME desktop-presence-display COMMAND whatsappgo --presence-display-test)
    set_tests_properties(desktop-light-theme desktop-dark-theme PROPERTIES
        ENVIRONMENT "QT_QPA_PLATFORM=offscreen"
        TIMEOUT 10
    )
    set_tests_properties(desktop-search-navigation PROPERTIES
        ENVIRONMENT "QT_QPA_PLATFORM=offscreen"
        TIMEOUT 10
    )
    set_tests_properties(desktop-message-interactions PROPERTIES
        ENVIRONMENT "QT_QPA_PLATFORM=offscreen"
        TIMEOUT 10
    )
    set_tests_properties(desktop-clipboard-images PROPERTIES
        ENVIRONMENT "QT_QPA_PLATFORM=offscreen"
        TIMEOUT 10
    )
    set_tests_properties(desktop-profile-removal PROPERTIES
        ENVIRONMENT "QT_QPA_PLATFORM=offscreen;XDG_DATA_HOME=${CMAKE_CURRENT_BINARY_DIR}/profile-removal-test/data;XDG_CACHE_HOME=${CMAKE_CURRENT_BINARY_DIR}/profile-removal-test/cache;XDG_CONFIG_HOME=${CMAKE_CURRENT_BINARY_DIR}/profile-removal-test/config"
        TIMEOUT 20
    )
    set_tests_properties(desktop-profile-removal-monitored PROPERTIES
        ENVIRONMENT "QT_QPA_PLATFORM=offscreen;WHATSAPPGO_DISABLE_PROFILE_MONITORS=0;WHATSAPPGO_BACKEND=/bin/false;XDG_DATA_HOME=${CMAKE_CURRENT_BINARY_DIR}/profile-removal-monitored/data;XDG_CACHE_HOME=${CMAKE_CURRENT_BINARY_DIR}/profile-removal-monitored/cache;XDG_CONFIG_HOME=${CMAKE_CURRENT_BINARY_DIR}/profile-removal-monitored/config"
        TIMEOUT 30
    )
    set_tests_properties(desktop-layout-regressions desktop-media-preview desktop-chat-filters desktop-chat-row-menu desktop-search-results PROPERTIES
        ENVIRONMENT "QT_QPA_PLATFORM=offscreen"
        TIMEOUT 10
    )
    set_tests_properties(desktop-resize-rendering desktop-message-layout desktop-message-scroll PROPERTIES
        ENVIRONMENT "QT_QPA_PLATFORM=offscreen"
        TIMEOUT 10
    )
    set_tests_properties(desktop-pairing-auto-start desktop-presence-updates desktop-link-preview-upgrade PROPERTIES TIMEOUT 5)
    set_tests_properties(desktop-system-integration PROPERTIES
        ENVIRONMENT "QT_QPA_PLATFORM=offscreen"
        TIMEOUT 10
    )
    set_tests_properties(desktop-contact-info PROPERTIES
        ENVIRONMENT "QT_QPA_PLATFORM=offscreen"
        TIMEOUT 10
    )
    set_tests_properties(desktop-status-stories PROPERTIES
        ENVIRONMENT "QT_QPA_PLATFORM=offscreen"
        TIMEOUT 10
    )
    set_tests_properties(desktop-presence-display PROPERTIES
        ENVIRONMENT "QT_QPA_PLATFORM=offscreen"
        TIMEOUT 10
    )
    set_tests_properties(desktop-backend-lifecycle PROPERTIES
        # AF_UNIX paths are limited to roughly 108 bytes on Linux. Keep the
        # runtime root deliberately short so an out-of-tree build directory
        # cannot make the daemon socket invalid.
        ENVIRONMENT "QT_QPA_PLATFORM=offscreen;XDG_DATA_HOME=${CMAKE_BINARY_DIR}/lifecycle-data;XDG_CACHE_HOME=${CMAKE_BINARY_DIR}/lifecycle-cache;XDG_CONFIG_HOME=${CMAKE_BINARY_DIR}/lifecycle-config;XDG_RUNTIME_DIR=${CMAKE_BINARY_DIR}/rt"
        TIMEOUT 10
    )
endif()
