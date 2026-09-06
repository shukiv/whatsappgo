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
    src/updateinstaller.cpp
    src/updateinstaller.h
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
    SOURCES
        src/videosurface.cpp
        src/videosurface.h
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
        qml/BugReportDialog.qml
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
        qml/icons/bug.svg
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

# The generated QML type registration includes <videosurface.h> by name.
target_include_directories(whatsappgo PRIVATE src)

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

set_target_properties(whatsappgo PROPERTIES
    # No console window behind the application on Windows. include(CTest)
    # below turns this off again for a testing tree, where the tests' own
    # output has to reach ctest.
    WIN32_EXECUTABLE ON
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
    # A Windows GUI subsystem binary has no console, so everything a --*-test
    # run writes to stdout or stderr is discarded and a failure reports nothing
    # at all. The release scripts configure with BUILD_TESTING=OFF and still
    # get a windowless binary.
    set_target_properties(whatsappgo PROPERTIES WIN32_EXECUTABLE OFF)

    # desktop-backend-lifecycle starts the real daemon, which the client looks
    # for beside itself. `make desktop` copies it there; a bare cmake build
    # never did, so the test could only pass in a tree built through the
    # Makefile. Copy it here too when it has been built.
    set(WHATSAPPGO_BUILT_DAEMON "${CMAKE_CURRENT_SOURCE_DIR}/../bin/whatsappd${CMAKE_EXECUTABLE_SUFFIX}")
    if(EXISTS "${WHATSAPPGO_BUILT_DAEMON}")
        add_custom_command(TARGET whatsappgo POST_BUILD
            COMMAND ${CMAKE_COMMAND} -E copy_if_different
                    "${WHATSAPPGO_BUILT_DAEMON}"
                    "$<TARGET_FILE_DIR:whatsappgo>/whatsappd${CMAKE_EXECUTABLE_SUFFIX}"
            VERBATIM
        )
    endif()
    qt_add_executable(profile-monitor-test
        tests/ProfileMonitor.cpp
        src/profilemonitor.cpp
        src/profilemonitor.h
    )
    target_include_directories(profile-monitor-test PRIVATE src)
    target_link_libraries(profile-monitor-test PRIVATE Qt6::Core Qt6::Network)
    # A Unix socket path cannot exceed 104 characters on macOS, and a runner's
    # build directory alone spends most of that: the tests that serve a socket
    # under one failed with "QLocalServer::listen: Name error". The suffix
    # keeps two build trees on one machine apart.
    if(UNIX)
        string(MD5 whatsappgo_build_hash "${CMAKE_BINARY_DIR}")
        string(SUBSTRING "${whatsappgo_build_hash}" 0 8 whatsappgo_build_hash)
        set(WHATSAPPGO_TEST_RUNTIME_BASE "/tmp/wag-${whatsappgo_build_hash}")
    else()
        set(WHATSAPPGO_TEST_RUNTIME_BASE "${CMAKE_BINARY_DIR}/rt")
    endif()

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
        src/updateinstaller.cpp
        src/updateinstaller.h
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
        src/updateinstaller.cpp
        src/updateinstaller.h
    )
    target_include_directories(presence-updates-test PRIVATE src)
    target_link_libraries(presence-updates-test PRIVATE Qt6::Core Qt6::Gui Qt6::Network)

    qt_add_executable(update-flow-test
        tests/UpdateFlow.cpp
        src/messagemodel.cpp
        src/messagemodel.h
        src/profilemonitor.cpp
        src/profilemonitor.h
        src/rpcclient.cpp
        src/rpcclient.h
        src/updateinstaller.cpp
        src/updateinstaller.h
    )
    target_include_directories(update-flow-test PRIVATE src)
    target_link_libraries(update-flow-test PRIVATE Qt6::Core Qt6::Gui Qt6::Network)

    qt_add_executable(chat-list-updates-test
        tests/ChatListUpdates.cpp
        src/messagemodel.cpp
        src/messagemodel.h
        src/profilemonitor.cpp
        src/profilemonitor.h
        src/rpcclient.cpp
        src/rpcclient.h
        src/updateinstaller.cpp
        src/updateinstaller.h
    )
    target_include_directories(chat-list-updates-test PRIVATE src)
    target_link_libraries(chat-list-updates-test PRIVATE Qt6::Core Qt6::Gui Qt6::Network)

    qt_add_executable(conversation-updates-test
        tests/ConversationUpdates.cpp
        src/messagemodel.cpp
        src/messagemodel.h
        src/profilemonitor.cpp
        src/profilemonitor.h
        src/rpcclient.cpp
        src/rpcclient.h
        src/updateinstaller.cpp
        src/updateinstaller.h
    )
    target_include_directories(conversation-updates-test PRIVATE src)
    target_link_libraries(conversation-updates-test PRIVATE Qt6::Core Qt6::Gui Qt6::Network)

    qt_add_executable(video-surface-test
        tests/VideoSurface.cpp
        src/videosurface.cpp
        src/videosurface.h
    )
    target_include_directories(video-surface-test PRIVATE src)
    target_link_libraries(video-surface-test PRIVATE Qt6::Core Qt6::Gui Qt6::Quick Qt6::Multimedia)

    qt_add_executable(notification-routing-test
        tests/NotificationRouting.cpp
        src/messagemodel.cpp
        src/messagemodel.h
        src/profilemonitor.cpp
        src/profilemonitor.h
        src/rpcclient.cpp
        src/rpcclient.h
        src/updateinstaller.cpp
        src/updateinstaller.h
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
        src/updateinstaller.cpp
        src/updateinstaller.h
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
        src/updateinstaller.cpp
        src/updateinstaller.h
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
        src/updateinstaller.cpp
        src/updateinstaller.h
    )
    target_include_directories(profile-display-names-test PRIVATE src)
    target_link_libraries(profile-display-names-test PRIVATE Qt6::Core Qt6::Gui Qt6::Network)
    add_test(NAME desktop-profile-display-names COMMAND profile-display-names-test)
    add_test(NAME desktop-conversation-updates COMMAND conversation-updates-test)
    add_test(NAME desktop-video-surface COMMAND video-surface-test)
    set_tests_properties(desktop-video-surface PROPERTIES ENVIRONMENT "QT_QPA_PLATFORM=offscreen")
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
    add_test(NAME desktop-menu-placement COMMAND whatsappgo --menu-placement-test)
    set_tests_properties(desktop-qml-smoke PROPERTIES
        ENVIRONMENT "QT_QPA_PLATFORM=offscreen"
        TIMEOUT 10
    )
    # The menus are measured on a window that has been laid out, so this one
    # shows a window. There is no display on CI, hence the offscreen platform.
    set_tests_properties(desktop-menu-placement PROPERTIES
        ENVIRONMENT "QT_QPA_PLATFORM=offscreen"
        TIMEOUT 20
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
    add_test(NAME desktop-update-flow COMMAND update-flow-test)
    set_tests_properties(desktop-update-flow PROPERTIES TIMEOUT 15)
    add_test(NAME desktop-link-preview-upgrade COMMAND link-preview-upgrade-test)
    add_test(NAME desktop-system-integration COMMAND whatsappgo --desktop-integration-test)
    add_test(NAME desktop-contact-info COMMAND whatsappgo --contact-info-test)
    add_test(NAME desktop-file-drop COMMAND whatsappgo --file-drop-test)
    # Every other window-opening test names the platform plugin; this one did
    # not, so it tried to reach a display and aborted on any headless machine.
    set_tests_properties(desktop-file-drop PROPERTIES ENVIRONMENT "QT_QPA_PLATFORM=offscreen")
    add_test(NAME desktop-status-stories COMMAND whatsappgo --status-stories-test)
    add_test(NAME desktop-file-urls COMMAND whatsappgo --file-url-test)
    set_tests_properties(desktop-file-urls PROPERTIES ENVIRONMENT "QT_QPA_PLATFORM=offscreen")
    add_test(NAME desktop-style-detection COMMAND whatsappgo --style-detection-test)
    set_tests_properties(desktop-style-detection PROPERTIES ENVIRONMENT "QT_QPA_PLATFORM=offscreen")
    add_test(NAME desktop-update-settings COMMAND whatsappgo --update-settings-test)
    set_tests_properties(desktop-update-settings PROPERTIES ENVIRONMENT "QT_QPA_PLATFORM=offscreen" TIMEOUT 15)
    add_test(NAME desktop-bug-report COMMAND whatsappgo --bug-report-test)
    set_tests_properties(desktop-bug-report PROPERTIES ENVIRONMENT "QT_QPA_PLATFORM=offscreen" TIMEOUT 15)
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
    # The search test stands up its own daemon, so it must not be able to reach
    # a real one: without its own runtime directory it connected to whatever
    # daemon the developer had running, and passed on their live account.
    set_tests_properties(desktop-search-results PROPERTIES
        ENVIRONMENT "QT_QPA_PLATFORM=offscreen;XDG_RUNTIME_DIR=${WHATSAPPGO_TEST_RUNTIME_BASE}/sr;XDG_DATA_HOME=${CMAKE_CURRENT_BINARY_DIR}/search-results-test/data;XDG_CACHE_HOME=${CMAKE_CURRENT_BINARY_DIR}/search-results-test/cache;XDG_CONFIG_HOME=${CMAKE_CURRENT_BINARY_DIR}/search-results-test/config"
    )
    set_tests_properties(desktop-layout-regressions desktop-media-preview desktop-chat-filters desktop-chat-row-menu PROPERTIES
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
        ENVIRONMENT "QT_QPA_PLATFORM=offscreen;XDG_DATA_HOME=${CMAKE_BINARY_DIR}/lifecycle-data;XDG_CACHE_HOME=${CMAKE_BINARY_DIR}/lifecycle-cache;XDG_CONFIG_HOME=${CMAKE_BINARY_DIR}/lifecycle-config;XDG_RUNTIME_DIR=${WHATSAPPGO_TEST_RUNTIME_BASE}/lc"
        TIMEOUT 10
    )
endif()
