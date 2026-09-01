cmake_minimum_required(VERSION 3.22)
project(WhatsAppGo VERSION 0.1.0 LANGUAGES CXX)

set(CMAKE_CXX_STANDARD 20)
set(CMAKE_CXX_STANDARD_REQUIRED ON)
set(CMAKE_AUTOMOC ON)
set(CMAKE_AUTORCC ON)

find_package(Qt6 6.5 REQUIRED COMPONENTS Core Gui Widgets Quick QuickControls2 Network Multimedia)
find_package(KF6Kirigami 6.0 REQUIRED)

qt_standard_project_setup(REQUIRES 6.5)
qt_policy(SET QTP0004 NEW)

qt_add_executable(whatsappgo
    src/main.cpp
    src/messagemodel.cpp
    src/messagemodel.h
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

set_source_files_properties(qml/Theme.qml PROPERTIES QT_QML_SINGLETON_TYPE TRUE)
set_source_files_properties(qml/Playback.qml PROPERTIES QT_QML_SINGLETON_TYPE TRUE)

qt_add_qml_module(whatsappgo
    URI org.whatsappgo
    VERSION 1.0
    NO_IMPORT_SCAN
    QML_FILES
        qml/Main.qml
        qml/Theme.qml
        qml/Playback.qml
        qml/PairingPage.qml
        qml/ChatListDelegate.qml
        qml/ChatFilterBar.qml
        qml/ChatFilterChip.qml
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
        qml/WhatsAppMenuPopup.qml
        qml/WhatsAppMenuItem.qml
        qml/VoiceRecorder.qml
        qml/MediaPreview.qml
        qml/ContactInfoDrawer.qml
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
        qml/icons/link.svg
        qml/icons/lock.svg
        qml/icons/chevron-right.svg
        qml/assets/chat-background.png
)

target_link_libraries(whatsappgo PRIVATE
    Qt6::Core Qt6::Gui Qt6::Widgets Qt6::Quick Qt6::QuickControls2 Qt6::Network Qt6::Multimedia
    KF6::Kirigami
)

target_compile_definitions(whatsappgo PRIVATE
    QT_NO_CAST_FROM_ASCII
    QT_NO_CAST_TO_ASCII
)

install(TARGETS whatsappgo RUNTIME DESTINATION bin)
install(PROGRAMS
    ${CMAKE_CURRENT_SOURCE_DIR}/../bin/whatsappd
    ${CMAKE_CURRENT_SOURCE_DIR}/../bin/whatsappctl
    DESTINATION bin
)
install(FILES ../packaging/applications/org.whatsappgo.Desktop.desktop
        DESTINATION share/applications)
install(FILES ../assets/org.whatsappgo.Desktop.svg
        DESTINATION share/icons/hicolor/scalable/apps)

include(CTest)
if(BUILD_TESTING)
    qt_add_executable(pairing-auto-start-test
        tests/PairingAutoStart.cpp
        src/messagemodel.cpp
        src/messagemodel.h
        src/rpcclient.cpp
        src/rpcclient.h
    )
    target_include_directories(pairing-auto-start-test PRIVATE src)
    target_link_libraries(pairing-auto-start-test PRIVATE Qt6::Core Qt6::Gui Qt6::Network)

    qt_add_executable(message-model-test
        tests/MessageModel.cpp
        src/messagemodel.cpp
        src/messagemodel.h
    )
    target_include_directories(message-model-test PRIVATE src)
    target_link_libraries(message-model-test PRIVATE Qt6::Core)
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
    add_test(NAME desktop-backend-lifecycle COMMAND whatsappgo --profile lifecycle-test --backend-lifecycle-test)
    add_test(NAME desktop-resize-rendering COMMAND whatsappgo --resize-rendering-test)
    add_test(NAME desktop-message-layout COMMAND whatsappgo --message-layout-test)
    add_test(NAME desktop-pairing-auto-start COMMAND pairing-auto-start-test)
    add_test(NAME desktop-system-integration COMMAND whatsappgo --desktop-integration-test)
    add_test(NAME desktop-contact-info COMMAND whatsappgo --contact-info-test)
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
    set_tests_properties(desktop-layout-regressions desktop-media-preview desktop-chat-filters PROPERTIES
        ENVIRONMENT "QT_QPA_PLATFORM=offscreen"
        TIMEOUT 10
    )
    set_tests_properties(desktop-resize-rendering desktop-message-layout PROPERTIES
        ENVIRONMENT "QT_QPA_PLATFORM=offscreen"
        TIMEOUT 10
    )
    set_tests_properties(desktop-pairing-auto-start PROPERTIES TIMEOUT 5)
    set_tests_properties(desktop-system-integration PROPERTIES
        ENVIRONMENT "QT_QPA_PLATFORM=offscreen"
        TIMEOUT 10
    )
    set_tests_properties(desktop-contact-info PROPERTIES
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
