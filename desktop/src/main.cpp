#include "rpcclient.h"
#include "traybehavior.h"

#include <QApplication>
#include <QAction>
#include <QClipboard>
#include <QCommandLineParser>
#include <QDebug>
#include <QDir>
#include <QEventLoop>
#include <QQmlApplicationEngine>
#include <QQmlComponent>
#include <QQmlContext>
#include <QQuickStyle>
#include <QLocalServer>
#include <QLocalSocket>
#include <QRegularExpression>
#include <QSettings>
#include <QStandardPaths>
#include <QTimer>
#include <QWindow>
#include <QQuickWindow>
#include <QImage>
#include <QIcon>
#include <QFileInfo>
#include <QFont>
#include <QMenu>
#include <QQuickItem>
#include <QProcess>
#include <QSystemTrayIcon>
#include <QTemporaryDir>

#include <cstdlib>

// QtTest's wheel helper enters through this QtGui window-system seam. Calling
// it here keeps the production target free of a QtTest dependency while the
// embedded headless regression still receives a genuine spontaneous event.
Q_GUI_EXPORT void qt_handleWheelEvent(QWindow *window, const QPointF &local,
                                      const QPointF &global, QPoint pixelDelta,
                                      QPoint angleDelta, Qt::KeyboardModifiers modifiers,
                                      Qt::ScrollPhase phase);

namespace {

void installResizeRepaintGuard(QQuickWindow *window)
{
    if (window == nullptr)
        return;

    // Qt Quick's software scene graph can occasionally leave stale backing-store
    // tiles behind when X11 resizes overlap a large QML state change (for example,
    // switching account data while dragging the window edge). Request an immediate
    // frame during the drag, then dirty the complete root item once the geometry
    // has settled so the final window cannot retain old or uninitialised tiles.
    auto *settledRepaint = new QTimer(window);
    settledRepaint->setObjectName(QStringLiteral("resizeRepaintTimer"));
    settledRepaint->setSingleShot(true);
    settledRepaint->setInterval(32);
    window->setProperty("_resizeRepaintGeneration", 0);

    const auto scheduleRepaint = [window, settledRepaint] {
        window->update();
        settledRepaint->start();
    };
    QObject::connect(window, &QWindow::widthChanged, window,
                     [scheduleRepaint](int) { scheduleRepaint(); });
    QObject::connect(window, &QWindow::heightChanged, window,
                     [scheduleRepaint](int) { scheduleRepaint(); });
    QObject::connect(settledRepaint, &QTimer::timeout, window, [window] {
        if (auto *rootItem = window->contentItem())
            rootItem->update();
        window->update();
        window->setProperty("_resizeRepaintGeneration",
                            window->property("_resizeRepaintGeneration").toInt() + 1);
    });
}

} // namespace

int main(int argc, char *argv[])
{
    // The client is a mostly static, low-memory UI. Qt's software scene graph
    // avoids fragile GLX/EGL setup on hybrid-GPU Linux systems and does not
    // create a persistent GPU context. Set QT_QUICK_BACKEND=rhi to opt back
    // into the hardware renderer on systems with a working graphics stack.
    if (qEnvironmentVariableIsEmpty("QT_QUICK_BACKEND"))
        qputenv("QT_QUICK_BACKEND", QByteArrayLiteral("software"));

    QApplication app(argc, argv);
    QApplication::setOrganizationName(QStringLiteral("WhatsAppGo"));
    QApplication::setOrganizationDomain(QStringLiteral("whatsappgo.org"));
    QApplication::setApplicationName(QStringLiteral("WhatsAppGo"));
    QApplication::setDesktopFileName(QStringLiteral("org.whatsappgo.Desktop"));
    const QIcon applicationIcon = QIcon::fromTheme(QStringLiteral("org.whatsappgo.Desktop"),
                                                   QIcon(QStringLiteral(":/org.whatsappgo.Desktop.svg")));
    QApplication::setWindowIcon(applicationIcon);
    // The same face stack WhatsApp Web asks for, resolved against whatever the
    // system actually has. Leaving it to Qt's default picked a heavier font
    // than the interface is designed around.
    {
        QFont interfaceFont = QGuiApplication::font();
        interfaceFont.setFamilies({
            QStringLiteral("Segoe UI"), QStringLiteral("Helvetica Neue"), QStringLiteral("Helvetica"),
            QStringLiteral("Lucida Grande"), QStringLiteral("Ubuntu"), QStringLiteral("Cantarell"),
            QStringLiteral("Fira Sans"), QStringLiteral("Noto Sans"), QStringLiteral("DejaVu Sans"),
            QStringLiteral("Arial"),
        });
        interfaceFont.setPixelSize(14);
        QGuiApplication::setFont(interfaceFont);
    }
    // Kirigami's desktop integration supplies the matching platform theme on
    // every Linux desktop; it does not require running the Plasma shell.
    // Users can still override it with QT_QUICK_CONTROLS_STYLE.
    if (qEnvironmentVariableIsEmpty("QT_QUICK_CONTROLS_STYLE"))
        QQuickStyle::setStyle(QStringLiteral("org.kde.desktop"));

    QCommandLineParser parser;
    parser.setApplicationDescription(QStringLiteral("Native low-memory WhatsApp client"));
    parser.addHelpOption();
    QCommandLineOption profileOption(QStringLiteral("profile"), QStringLiteral("Open an isolated account profile"), QStringLiteral("name"));
    parser.addOption(profileOption);
    QCommandLineOption chatOption(QStringLiteral("chat"), QStringLiteral("Open a conversation JID"), QStringLiteral("jid"));
    parser.addOption(chatOption);
    QCommandLineOption smokeTestOption(QStringLiteral("smoke-test"), QStringLiteral("Load the complete QML interface and exit"));
    parser.addOption(smokeTestOption);
    QCommandLineOption searchNavigationTestOption(QStringLiteral("search-navigation-test"), QStringLiteral("Verify that the rail search action opens and focuses chat search"));
    parser.addOption(searchNavigationTestOption);
    QCommandLineOption messageInteractionTestOption(QStringLiteral("message-interaction-test"), QStringLiteral("Verify message text can be selected and links are interactive"));
    parser.addOption(messageInteractionTestOption);
    QCommandLineOption clipboardImageTestOption(QStringLiteral("clipboard-image-test"), QStringLiteral("Verify native image copy and paste preparation"));
    parser.addOption(clipboardImageTestOption);
    QCommandLineOption layoutRegressionTestOption(QStringLiteral("layout-regression-test"), QStringLiteral("Verify unread badges and multiline message geometry"));
    parser.addOption(layoutRegressionTestOption);
    QCommandLineOption mediaPreviewTestOption(QStringLiteral("media-preview-test"), QStringLiteral("Verify the native media preview composer"));
    parser.addOption(mediaPreviewTestOption);
    QCommandLineOption chatFilterTestOption(QStringLiteral("chat-filter-test"), QStringLiteral("Verify chat filter controls and filtering behavior"));
    parser.addOption(chatFilterTestOption);
    QCommandLineOption backendLifecycleTestOption(QStringLiteral("backend-lifecycle-test"), QStringLiteral("Verify that the desktop owns its bundled backend process"));
    parser.addOption(backendLifecycleTestOption);
    QCommandLineOption resizeRenderingTestOption(QStringLiteral("resize-rendering-test"), QStringLiteral("Verify that resizing schedules a complete scene repaint"));
    parser.addOption(resizeRenderingTestOption);
    QCommandLineOption messageLayoutTestOption(QStringLiteral("message-layout-test"), QStringLiteral("Verify message bubble width limits, padding, and media previews"));
    parser.addOption(messageLayoutTestOption);
    QCommandLineOption messageScrollTestOption(QStringLiteral("message-scroll-test"), QStringLiteral("Verify conversations open and remain anchored at the newest message"));
    parser.addOption(messageScrollTestOption);
    QCommandLineOption desktopIntegrationTestOption(QStringLiteral("desktop-integration-test"), QStringLiteral("Verify system tray and notification integration"));
    parser.addOption(desktopIntegrationTestOption);
    QCommandLineOption contactInfoTestOption(QStringLiteral("contact-info-test"), QStringLiteral("Verify the contact information and shared-content drawer"));
    parser.addOption(contactInfoTestOption);
    QCommandLineOption statusStoriesTestOption(QStringLiteral("status-stories-test"), QStringLiteral("Verify grouped status rows and sequential story playback"));
    parser.addOption(statusStoriesTestOption);
    QCommandLineOption screenshotOption(QStringLiteral("screenshot"), QStringLiteral("Render the interface to a PNG and exit"), QStringLiteral("path"));
    parser.addOption(screenshotOption);
    QCommandLineOption themeOption(QStringLiteral("theme"), QStringLiteral("Override the appearance for this run (system, light, or dark)"), QStringLiteral("mode"));
    parser.addOption(themeOption);
    QCommandLineOption sectionOption(QStringLiteral("section"), QStringLiteral("Open a primary section (chats, status, calls, channels, communities, or profile)"), QStringLiteral("name"));
    parser.addOption(sectionOption);
    parser.process(app);
    const bool smokeTest = parser.isSet(smokeTestOption);
    const bool searchNavigationTest = parser.isSet(searchNavigationTestOption);
    const bool messageInteractionTest = parser.isSet(messageInteractionTestOption);
    const bool clipboardImageTest = parser.isSet(clipboardImageTestOption);
    const bool layoutRegressionTest = parser.isSet(layoutRegressionTestOption);
    const bool mediaPreviewTest = parser.isSet(mediaPreviewTestOption);
    const bool chatFilterTest = parser.isSet(chatFilterTestOption);
    const bool backendLifecycleTest = parser.isSet(backendLifecycleTestOption);
    const bool resizeRenderingTest = parser.isSet(resizeRenderingTestOption);
    const bool messageLayoutTest = parser.isSet(messageLayoutTestOption);
    const bool messageScrollTest = parser.isSet(messageScrollTestOption);
    const bool desktopIntegrationTest = parser.isSet(desktopIntegrationTestOption);
    const bool contactInfoTest = parser.isSet(contactInfoTestOption);
    const bool statusStoriesTest = parser.isSet(statusStoriesTestOption);
    const auto screenshotPath = parser.value(screenshotOption);
    const bool automatedRun = smokeTest || searchNavigationTest || messageInteractionTest || clipboardImageTest
        || layoutRegressionTest || mediaPreviewTest || chatFilterTest || backendLifecycleTest || resizeRenderingTest
        || messageLayoutTest || messageScrollTest || desktopIntegrationTest || contactInfoTest || statusStoriesTest
        || !screenshotPath.isEmpty();

    if (automatedRun)
        qputenv("WHATSAPPGO_DISABLE_PROFILE_MONITORS", "1");

    auto initialProfile = parser.value(profileOption);
    const QRegularExpression validProfile(QStringLiteral("^[a-z0-9][a-z0-9_-]{0,31}$"));
    if (!validProfile.match(initialProfile).hasMatch())
        initialProfile = QSettings().value(QStringLiteral("accounts/current"), QStringLiteral("default")).toString();
    if (!validProfile.match(initialProfile).hasMatch())
        initialProfile = QStringLiteral("default");
    auto runtime = qEnvironmentVariable("XDG_RUNTIME_DIR");
    if (runtime.isEmpty())
        runtime = QStandardPaths::writableLocation(QStandardPaths::RuntimeLocation);
    const auto uiRuntime = QDir(runtime).filePath(QStringLiteral("whatsappgo"));
    QDir().mkpath(uiRuntime);
    const auto instanceSocket = QDir(uiRuntime).filePath(QStringLiteral("ui-%1.sock").arg(initialProfile));
    QLocalServer instanceServer;
    if (!automatedRun) {
        QLocalSocket existingInstance;
        existingInstance.connectToServer(instanceSocket, QIODevice::WriteOnly);
        if (existingInstance.waitForConnected(150)) {
            existingInstance.write(parser.value(chatOption).toUtf8() + '\n');
            existingInstance.waitForBytesWritten(150);
            return EXIT_SUCCESS;
        }
        QLocalServer::removeServer(instanceSocket);
        instanceServer.setSocketOptions(QLocalServer::UserAccessOption);
        if (!instanceServer.listen(instanceSocket))
            return EXIT_FAILURE;
    }

    bool trayAvailable = !automatedRun && QSystemTrayIcon::isSystemTrayAvailable();
    if (!automatedRun)
        app.setQuitOnLastWindowClosed(!TrayBehavior::shouldKeepRunning(trayAvailable));

    RpcClient backend(initialProfile, parser.value(chatOption));
    QQmlApplicationEngine engine;
    engine.rootContext()->setContextProperty(QStringLiteral("backend"), &backend);
    QObject::connect(&engine, &QQmlApplicationEngine::objectCreationFailed, &app,
                     [] { QCoreApplication::exit(EXIT_FAILURE); }, Qt::QueuedConnection);
    engine.loadFromModule(QStringLiteral("org.whatsappgo"), QStringLiteral("Main"));
    auto *applicationWindow = engine.rootObjects().isEmpty()
        ? nullptr
        : qobject_cast<QQuickWindow *>(engine.rootObjects().constFirst());
    if (applicationWindow != nullptr)
        installResizeRepaintGuard(applicationWindow);

    // Qt keeps a visible QSystemTrayIcon registered if a tray host appears
    // after startup. Build it for every interactive run, but never for the
    // offscreen test platform, whose widget teardown is not tray-safe.
    std::unique_ptr<QMenu> trayMenu;
    std::unique_ptr<QSystemTrayIcon> trayIcon;
    QTimer trayAvailabilityTimer;
    if (!automatedRun) {
        trayMenu = std::make_unique<QMenu>();
        trayMenu->setObjectName(QStringLiteral("whatsappgoTrayMenu"));
        auto *trayStatusAction = trayMenu->addAction(QStringLiteral("Connecting…"));
        trayStatusAction->setObjectName(QStringLiteral("trayStatusAction"));
        trayStatusAction->setEnabled(false);
        trayMenu->addSeparator();
        auto *trayToggleAction = trayMenu->addAction(QStringLiteral("Hide WhatsAppGo"));
        trayToggleAction->setObjectName(QStringLiteral("trayToggleAction"));
        trayMenu->addSeparator();
        auto *trayQuitAction = trayMenu->addAction(QStringLiteral("Quit WhatsAppGo"));
        trayQuitAction->setObjectName(QStringLiteral("trayQuitAction"));

        trayIcon = std::make_unique<QSystemTrayIcon>(applicationIcon);
        trayIcon->setObjectName(QStringLiteral("whatsappgoTrayIcon"));
        trayIcon->setContextMenu(trayMenu.get());
        trayIcon->setToolTip(QStringLiteral("WhatsAppGo"));

        const auto activateWindow = [applicationWindow] {
            if (applicationWindow == nullptr)
                return;
            applicationWindow->show();
            applicationWindow->raise();
            applicationWindow->requestActivate();
        };
        const auto updateToggleAction = [applicationWindow, trayToggleAction] {
            trayToggleAction->setText(applicationWindow != nullptr && applicationWindow->isVisible()
                                          ? QStringLiteral("Hide WhatsAppGo")
                                          : QStringLiteral("Open WhatsAppGo"));
        };
        QObject::connect(trayToggleAction, &QAction::triggered, &app, [applicationWindow, activateWindow, updateToggleAction] {
            if (applicationWindow == nullptr)
                return;
            if (applicationWindow->isVisible())
                applicationWindow->hide();
            else
                activateWindow();
            updateToggleAction();
        });
        QObject::connect(trayQuitAction, &QAction::triggered, &app, &QApplication::quit);
        QObject::connect(trayIcon.get(), &QSystemTrayIcon::activated, &app,
                         [activateWindow](QSystemTrayIcon::ActivationReason reason) {
                             if (reason == QSystemTrayIcon::Trigger || reason == QSystemTrayIcon::DoubleClick)
                                 activateWindow();
                         });
        const auto updateTrayStatus = [&backend, icon = trayIcon.get(), trayStatusAction] {
            const auto status = backend.status();
            const bool connected = status.value(QStringLiteral("connected")).toBool();
            const auto label = connected ? QStringLiteral("Connected") : QStringLiteral("Disconnected");
            trayStatusAction->setText(label);
            icon->setToolTip(QStringLiteral("WhatsAppGo — %1").arg(label));
        };
        QObject::connect(&backend, &RpcClient::statusChanged, &app, updateTrayStatus);
        QObject::connect(&backend, &RpcClient::daemonConnectedChanged, &app, updateTrayStatus);
        QObject::connect(&app, &QCoreApplication::aboutToQuit, trayIcon.get(), &QSystemTrayIcon::hide);
        const auto syncTrayAvailability = [&app, applicationWindow, activateWindow, icon = trayIcon.get(), &trayAvailable] {
            const bool availableNow = QSystemTrayIcon::isSystemTrayAvailable();
            if (!availableNow && trayAvailable && applicationWindow != nullptr && !applicationWindow->isVisible())
                activateWindow();
            trayAvailable = availableNow;
            app.setQuitOnLastWindowClosed(!TrayBehavior::shouldKeepRunning(trayAvailable));
            if (TrayBehavior::shouldHideWindow(applicationWindow == nullptr
                                                    ? QWindow::Hidden
                                                    : applicationWindow->visibility(),
                                                trayAvailable)) {
                QTimer::singleShot(0, applicationWindow, &QWindow::hide);
            }
            if (!icon->isVisible())
                icon->show();
        };
        if (applicationWindow != nullptr) {
            QObject::connect(applicationWindow, &QWindow::visibilityChanged, &app,
                             [applicationWindow, updateToggleAction, &trayAvailable](QWindow::Visibility visibility) {
                updateToggleAction();
                if (TrayBehavior::shouldHideWindow(visibility, trayAvailable))
                    QTimer::singleShot(0, applicationWindow, &QWindow::hide);
            });
        }
        trayAvailabilityTimer.setInterval(1500);
        QObject::connect(&trayAvailabilityTimer, &QTimer::timeout, &app, syncTrayAvailability);
        trayAvailabilityTimer.start();
        updateToggleAction();
        updateTrayStatus();
        trayIcon->show();
        syncTrayAvailability();
    }

    if (desktopIntegrationTest) {
        return !applicationIcon.isNull() && !trayAvailable && trayIcon == nullptr && trayMenu == nullptr
            ? EXIT_SUCCESS : EXIT_FAILURE;
    }
    if (contactInfoTest) {
        QQmlComponent component(&engine, QUrl(QStringLiteral("qrc:/qt/qml/org/whatsappgo/qml/ContactInfoDrawer.qml")));
        const QVariantMap chat{
            {QStringLiteral("jid"), QStringLiteral("573133878085@s.whatsapp.net")},
            {QStringLiteral("title"), QStringLiteral("Test contact")},
            {QStringLiteral("avatar_path"), QStringLiteral("/tmp/test-contact-round-123.png")},
            {QStringLiteral("muted_until"), 0},
            {QStringLiteral("favorite"), false},
            {QStringLiteral("archived"), false},
        };
        const QVariantMap info{
            {QStringLiteral("chat"), chat},
            {QStringLiteral("phone"), QStringLiteral("573133878085")},
            {QStringLiteral("shared_count"), 3},
            {QStringLiteral("media_count"), 1},
            {QStringLiteral("document_count"), 1},
            {QStringLiteral("link_count"), 1},
            {QStringLiteral("preview"), QVariantList{}},
        };
        const QVariantList sharedContent{
            QVariantMap{{QStringLiteral("id"), QStringLiteral("photo-1")},
                        {QStringLiteral("kind"), QStringLiteral("image")},
                        {QStringLiteral("media_path"), QStringLiteral("qrc:/qt/qml/org/whatsappgo/qml/assets/chat-background.png")}},
            QVariantMap{{QStringLiteral("id"), QStringLiteral("photo-2")},
                        {QStringLiteral("kind"), QStringLiteral("image")},
                        {QStringLiteral("media_path"), QStringLiteral("qrc:/qt/qml/org/whatsappgo/qml/assets/chat-background.png")}},
        };
        std::unique_ptr<QObject> drawer(component.createWithInitialProperties({
            {QStringLiteral("opened"), true},
            {QStringLiteral("width"), 440},
            {QStringLiteral("height"), 800},
            {QStringLiteral("selectedChat"), chat},
            {QStringLiteral("info"), info},
            {QStringLiteral("sharedContent"), sharedContent},
        }));
        if (!drawer)
            return EXIT_FAILURE;
        QCoreApplication::processEvents();
        auto *back = qobject_cast<QQuickItem *>(drawer->findChild<QObject *>(QStringLiteral("contactInfoBackButton")));
        auto *avatarButton = qobject_cast<QQuickItem *>(drawer->findChild<QObject *>(QStringLiteral("contactAvatarButton")));
        QVariant originalAvatarPath;
        const bool resolvedOriginalAvatar = QMetaObject::invokeMethod(
            drawer.get(), "originalAvatarPath", Q_RETURN_ARG(QVariant, originalAvatarPath),
            Q_ARG(QVariant, QStringLiteral("/tmp/test-contact-round-123.png")));
        const bool avatarClicked = avatarButton && QMetaObject::invokeMethod(avatarButton, "click");
        drawer->setProperty("sharedView", true);
        drawer->setProperty("activeCategory", QStringLiteral("media"));
        QCoreApplication::processEvents();
        auto *drawerItem = qobject_cast<QQuickItem *>(drawer.get());
        auto *grid = qobject_cast<QQuickItem *>(drawer->findChild<QObject *>(QStringLiteral("contactMediaGrid")));
        const auto gridTop = grid && drawerItem ? grid->mapToItem(drawerItem, QPointF(0, 0)).y() : -1.0;
        const auto gridHeight = grid ? grid->height() : -1.0;
        const auto gridCount = grid ? grid->property("count").toInt() : -1;
        qInfo().noquote() << QStringLiteral("contact shared grid top=%1 height=%2 count=%3")
                                .arg(gridTop).arg(gridHeight).arg(gridCount);

        drawer->setProperty("activeCategory", QStringLiteral("links"));
        QCoreApplication::processEvents();
        auto *list = qobject_cast<QQuickItem *>(drawer->findChild<QObject *>(QStringLiteral("contactSharedList")));
        const auto listTop = list && drawerItem ? list->mapToItem(drawerItem, QPointF(0, 0)).y() : -1.0;
        return back != nullptr && back->width() >= 44 && back->height() >= 44
                && avatarButton && avatarButton->width() >= 144 && avatarButton->height() >= 144
                && resolvedOriginalAvatar && originalAvatarPath.toString() == QStringLiteral("/tmp/test-contact.jpg")
                && avatarClicked
                && drawer->property("sharedView").toBool()
                && drawer->property("activeCategory").toString() == QStringLiteral("links")
                && drawer->property("width").toReal() >= 320
                && gridCount == 2
                && gridTop >= 120.0 && gridTop <= 150.0 && gridHeight >= 600.0
                && list != nullptr && list->property("count").toInt() == 2
                && listTop >= 120.0 && listTop <= 150.0 && list->height() >= 600.0
            ? EXIT_SUCCESS
            : EXIT_FAILURE;
    }
    if (statusStoriesTest) {
        const QVariantList aliceItems{
            QVariantMap{{QStringLiteral("id"), QStringLiteral("alice-1")},
                        {QStringLiteral("kind"), QStringLiteral("text")},
                        {QStringLiteral("body"), QStringLiteral("first")},
                        {QStringLiteral("timestamp"), 1}},
            QVariantMap{{QStringLiteral("id"), QStringLiteral("alice-2")},
                        {QStringLiteral("kind"), QStringLiteral("image")},
                        {QStringLiteral("media_thumbnail"), QStringLiteral("qrc:/qt/qml/org/whatsappgo/qml/assets/chat-background.png")},
                        {QStringLiteral("timestamp"), 2}},
        };
        const QVariantList statusGroups{
            QVariantMap{{QStringLiteral("sender_jid"), QStringLiteral("alice@lid")},
                        {QStringLiteral("sender_name"), QStringLiteral("Alice")},
                        {QStringLiteral("avatar_path"), QStringLiteral("/tmp/alice.jpg")},
                        {QStringLiteral("latest_at"), 2},
                        {QStringLiteral("items"), aliceItems}},
            QVariantMap{{QStringLiteral("sender_jid"), QStringLiteral("bob@lid")},
                        {QStringLiteral("sender_name"), QStringLiteral("Bob")},
                        {QStringLiteral("latest_at"), 3},
                        {QStringLiteral("items"), QVariantList{
                            QVariantMap{{QStringLiteral("id"), QStringLiteral("bob-1")},
                                        {QStringLiteral("kind"), QStringLiteral("text")},
                                        {QStringLiteral("body"), QStringLiteral("second person")},
                                        {QStringLiteral("timestamp"), 3}},
                        }}},
        };

        QQmlComponent pageComponent(&engine, QUrl(QStringLiteral("qrc:/qt/qml/org/whatsappgo/qml/StatusPage.qml")));
        std::unique_ptr<QObject> page(pageComponent.createWithInitialProperties({
            {QStringLiteral("width"), 1200}, {QStringLiteral("height"), 800},
            {QStringLiteral("groups"), statusGroups},
        }));
        QQmlComponent viewerComponent(&engine, QUrl(QStringLiteral("qrc:/qt/qml/org/whatsappgo/qml/StatusViewer.qml")));
        std::unique_ptr<QObject> viewer(viewerComponent.createWithInitialProperties({
            {QStringLiteral("width"), 1200}, {QStringLiteral("height"), 800},
            {QStringLiteral("groups"), statusGroups}, {QStringLiteral("opened"), true},
        }));
        QQmlComponent chatStatusComponent(&engine,
            QUrl(QStringLiteral("qrc:/qt/qml/org/whatsappgo/qml/ChatListDelegate.qml")));
        std::unique_ptr<QObject> chatStatus(chatStatusComponent.createWithInitialProperties({
            {QStringLiteral("width"), 436}, {QStringLiteral("height"), 72},
            {QStringLiteral("modelData"), QVariantMap{
                {QStringLiteral("jid"), QStringLiteral("bob@lid")},
                {QStringLiteral("title"), QStringLiteral("Bob")},
                {QStringLiteral("unread_count"), 0},
            }},
            {QStringLiteral("current"), false},
            {QStringLiteral("statusGroupIndex"), 1},
            {QStringLiteral("statusItemCount"), 2},
        }));
        if (!page || !viewer || !chatStatus)
            return EXIT_FAILURE;
        QCoreApplication::processEvents();
        auto *list = page->findChild<QObject *>(QStringLiteral("statusGroupList"));
        auto *statusAvatar = qobject_cast<QQuickItem *>(chatStatus->findChild<QObject *>(QStringLiteral("chatStatusAvatar")));
        auto *statusButton = qobject_cast<QQuickItem *>(chatStatus->findChild<QObject *>(QStringLiteral("chatStatusButton")));
        const bool openedStatusFromChat = statusButton && QMetaObject::invokeMethod(statusButton, "click");
        QCoreApplication::processEvents();
        const bool firstAdvance = QMetaObject::invokeMethod(viewer.get(), "advance");
        QCoreApplication::processEvents();
        const bool stayedWithAlice = viewer->property("groupIndex").toInt() == 0
            && viewer->property("itemIndex").toInt() == 1;
        const bool secondAdvance = QMetaObject::invokeMethod(viewer.get(), "advance");
        QCoreApplication::processEvents();
        const bool movedToBob = viewer->property("groupIndex").toInt() == 1
            && viewer->property("itemIndex").toInt() == 0;
        auto *replyComposer = viewer->findChild<QObject *>(QStringLiteral("statusReplyComposer"));
        viewer->setProperty("replyText", QStringLiteral("Looks great"));
        const bool submittedReply = QMetaObject::invokeMethod(viewer.get(), "submitReply");
        QCoreApplication::processEvents();
        const bool replyTargetsCurrentStatus = viewer->property("lastReplyRecipient").toString() == QStringLiteral("bob@lid")
            && viewer->property("lastReplyStatusId").toString() == QStringLiteral("bob-1")
            && viewer->property("lastReplyText").toString() == QStringLiteral("Looks great");
        return list && list->property("count").toInt() == 2
                && firstAdvance && stayedWithAlice && secondAdvance && movedToBob
                && replyComposer && replyComposer->property("visible").toBool()
                && submittedReply && replyTargetsCurrentStatus
                && statusAvatar && statusAvatar->isVisible()
                && statusButton->width() >= 44 && statusButton->height() >= 44
                && openedStatusFromChat && chatStatus->property("statusGroupIndex").toInt() == 1
            ? EXIT_SUCCESS : EXIT_FAILURE;
    }
    if (resizeRenderingTest) {
        if (engine.rootObjects().isEmpty())
            return EXIT_FAILURE;
        auto *window = qobject_cast<QQuickWindow *>(engine.rootObjects().constFirst());
        if (window == nullptr)
            return EXIT_FAILURE;
        const auto initialGeneration = window->property("_resizeRepaintGeneration").toInt();
        window->resize(window->width() - 37, window->height() - 23);
        QTimer::singleShot(150, &app, [window, initialGeneration] {
            const bool repainted = window->property("_resizeRepaintGeneration").toInt() > initialGeneration;
            QCoreApplication::exit(repainted ? EXIT_SUCCESS : EXIT_FAILURE);
        });
        return app.exec();
    }
    if (searchNavigationTest) {
        if (engine.rootObjects().isEmpty())
            return EXIT_FAILURE;
        auto *root = engine.rootObjects().constFirst();
        root->setProperty("activeSection", QStringLiteral("communities"));
        const bool invoked = QMetaObject::invokeMethod(root, "openChatSearch");
        QCoreApplication::processEvents();
        auto *field = root->findChild<QObject *>(QStringLiteral("chatSearchField"));
        return invoked && root->property("activeSection").toString() == QStringLiteral("chats")
                && field != nullptr && field->property("activeFocus").toBool()
            ? EXIT_SUCCESS
            : EXIT_FAILURE;
    }
    if (messageInteractionTest) {
        QQmlComponent component(&engine);
        component.setData(R"QML(
            import QtQuick
            import QtQuick.Controls
            import org.whatsappgo
            ApplicationWindow {
                id: harness
                width: 900
                height: 600
                visible: true
                property var testMessage
                property var imageMessage
                property string previewedMessageId: ""
                property string previewedMediaPath: ""
                property string quotedMessageId: ""
                MessageDelegate {
                    objectName: "messageInteractionDelegate"
                    width: 800
                    modelData: harness.testMessage
                    onQuotedMessageRequested: messageId => harness.quotedMessageId = messageId
                }
                MessageDelegate {
                    objectName: "imageInteractionDelegate"
                    width: 800
                    modelData: harness.imageMessage
                    onImagePreviewRequested: message => {
                        harness.previewedMessageId = String(message.id || "")
                        harness.previewedMediaPath = String(message.media_path || "")
                    }
                }
            }
        )QML", QUrl(QStringLiteral("qrc:/message-interaction-test.qml")));
        const QVariantMap message{
            {QStringLiteral("id"), QStringLiteral("message-test")},
            {QStringLiteral("body"), QStringLiteral("Copy this https://example.com/path?q=1")},
            {QStringLiteral("kind"), QStringLiteral("text")},
            {QStringLiteral("from_me"), false},
            {QStringLiteral("timestamp"), 0},
            {QStringLiteral("status"), QStringLiteral("received")},
            {QStringLiteral("reply_to"), QStringLiteral("quoted-message-test")},
            {QStringLiteral("reply_sender"), QStringLiteral("Alice")},
            {QStringLiteral("reply_preview"), QStringLiteral("Original message")},
            {QStringLiteral("reactions"), QVariantList{
                QVariantMap{{QStringLiteral("sender_jid"), QStringLiteral("someone@lid")},
                            {QStringLiteral("emoji"), QStringLiteral("🙏")}},
                QVariantMap{{QStringLiteral("sender_jid"), QStringLiteral("another@lid")},
                            {QStringLiteral("emoji"), QStringLiteral("👍")}},
                QVariantMap{{QStringLiteral("sender_jid"), QStringLiteral("third@lid")},
                            {QStringLiteral("emoji"), QStringLiteral("👍")}},
            }},
        };
        std::unique_ptr<QObject> harness(component.createWithInitialProperties({
            {QStringLiteral("testMessage"), message},
            {QStringLiteral("imageMessage"), QVariantMap{
                {QStringLiteral("id"), QStringLiteral("image-test")},
                {QStringLiteral("kind"), QStringLiteral("image")},
                {QStringLiteral("media_path"), QStringLiteral("/tmp/whatsappgo-image-test.jpg")},
                {QStringLiteral("from_me"), false},
                {QStringLiteral("timestamp"), 0},
                {QStringLiteral("status"), QStringLiteral("received")},
            }},
        }));
        if (!harness)
            return EXIT_FAILURE;
        QCoreApplication::processEvents();
        auto *delegate = harness->findChild<QObject *>(QStringLiteral("messageInteractionDelegate"));
        if (!delegate)
            return EXIT_FAILURE;
        auto *body = delegate->findChild<QObject *>(QStringLiteral("messageBody"));
        if (!body)
            return EXIT_FAILURE;
        auto *menuButton = qobject_cast<QQuickItem *>(delegate->findChild<QObject *>(QStringLiteral("messageMenuButton")));
        auto *reactionButton = qobject_cast<QQuickItem *>(delegate->findChild<QObject *>(QStringLiteral("messageReactionButton")));
        auto *reactionBadge = qobject_cast<QQuickItem *>(delegate->findChild<QObject *>(QStringLiteral("messageReactionBadge")));
        auto *reactionSummary = delegate->findChild<QObject *>(QStringLiteral("messageReactionSummary"));
        auto *bubble = qobject_cast<QQuickItem *>(delegate->findChild<QObject *>(QStringLiteral("messageBubble")));
        auto *menu = delegate->findChild<QObject *>(QStringLiteral("messageContextMenu"));
        auto *quickReactions = delegate->findChild<QObject *>(QStringLiteral("quickReactionPopup"));
        auto *quotedMessagePreview = qobject_cast<QQuickItem *>(
            delegate->findChild<QObject *>(QStringLiteral("quotedMessagePreview")));
        auto *imageDelegate = harness->findChild<QObject *>(QStringLiteral("imageInteractionDelegate"));
        const bool imagePreviewInvoked = imageDelegate
            && QMetaObject::invokeMethod(imageDelegate, "openVisualMedia");
        QCoreApplication::processEvents();
        const auto rendered = body->property("text").toString();
        const bool selected = QMetaObject::invokeMethod(body, "selectAll")
            && !body->property("selectedText").toString().isEmpty();
        const bool menuOpened = menuButton && QMetaObject::invokeMethod(menuButton, "click");
        QCoreApplication::processEvents();
        const bool quoteClicked = quotedMessagePreview
            && QMetaObject::invokeMethod(quotedMessagePreview, "click");
        QCoreApplication::processEvents();
        qInfo().noquote() << QStringLiteral("message interaction: selected=%1 link=%2 menuButton=%3 reactionButton=%4 badge=%5 summary=%6 badgeY=%7 bubbleH=%8 implicitH=%9 invoked=%10 menu=%11 reactions=%12")
                                 .arg(selected).arg(rendered.contains(QStringLiteral("href=\"https://example.com/path?q=1\"")))
                                 .arg(menuButton != nullptr).arg(reactionButton != nullptr)
                                 .arg(reactionBadge && reactionBadge->isVisible())
                                 .arg(reactionSummary ? reactionSummary->property("text").toString() : QStringLiteral("missing"))
                                 .arg(reactionBadge ? reactionBadge->y() : -1).arg(bubble ? bubble->height() : -1)
                                 .arg(delegate->property("implicitHeight").toReal()).arg(menuOpened)
                                 .arg(menu ? QStringLiteral("opened:%1 visible:%2 parent:%3")
                                                 .arg(menu->property("opened").toBool())
                                                 .arg(menu->property("visible").toBool())
                                                 .arg(menu->property("parent").value<QObject *>() != nullptr)
                                           : QStringLiteral("missing"))
                                 .arg(quickReactions ? QStringLiteral("opened:%1 visible:%2 parent:%3")
                                                          .arg(quickReactions->property("opened").toBool())
                                                          .arg(quickReactions->property("visible").toBool())
                                                          .arg(quickReactions->property("parent").value<QObject *>() != nullptr)
                                                    : QStringLiteral("missing"));
        return body->property("selectByMouse").toBool()
                && selected
                && rendered.contains(QStringLiteral("href=\"https://example.com/path?q=1\""))
                && menuButton && menuButton->width() >= 32 && menuButton->height() >= 32
                && reactionButton && reactionButton->width() >= 36 && reactionButton->height() >= 36
                && reactionBadge && reactionBadge->isVisible() && bubble
                && reactionSummary && reactionSummary->property("text").toString() == QStringLiteral("🙏  👍 2")
                && reactionBadge->y() >= bubble->height() - 6 && delegate->property("implicitHeight").toReal() > bubble->height()
                && menuOpened && menu && menu->property("visible").toBool()
                && quickReactions && quickReactions->property("visible").toBool()
                && quotedMessagePreview && quotedMessagePreview->isVisible()
                && quotedMessagePreview->height() >= 44 && quoteClicked
                && harness->property("quotedMessageId").toString() == QStringLiteral("quoted-message-test")
                && imagePreviewInvoked
                && harness->property("previewedMessageId").toString() == QStringLiteral("image-test")
                && harness->property("previewedMediaPath").toString() == QStringLiteral("/tmp/whatsappgo-image-test.jpg")
            ? EXIT_SUCCESS
            : EXIT_FAILURE;
    }
    if (layoutRegressionTest) {
        QQmlComponent accountComponent(&engine, QUrl(QStringLiteral("qrc:/qt/qml/org/whatsappgo/qml/AccountSwitcherButton.qml")));
        std::unique_ptr<QObject> accountChip(accountComponent.createWithInitialProperties({
            {QStringLiteral("profiles"), QStringList{QStringLiteral("default"), QStringLiteral("work")}},
            {QStringLiteral("currentProfile"), QStringLiteral("work")},
            {QStringLiteral("displayNames"), QVariantMap{{QStringLiteral("default"), QStringLiteral("Personal")},
                                                          {QStringLiteral("work"), QStringLiteral("Support")}}},
            {QStringLiteral("unreadCounts"), QVariantMap{{QStringLiteral("default"), 12}, {QStringLiteral("work"), 0}}},
        }));
        if (!accountChip)
            return EXIT_FAILURE;
        QQuickWindow accountWindow;
        accountWindow.resize(640, 480);
        if (auto *accountItem = qobject_cast<QQuickItem *>(accountChip.get()))
            accountItem->setParentItem(accountWindow.contentItem());
        accountWindow.show();
        QCoreApplication::processEvents();
        auto *accountUnreadBadge = qobject_cast<QQuickItem *>(accountChip->findChild<QObject *>(QStringLiteral("accountSwitcherUnreadBadge")));
        auto *accountChipItem = qobject_cast<QQuickItem *>(accountChip.get());
        auto *accountMenu = accountChip->findChild<QObject *>(QStringLiteral("accountSwitcherMenu"));
        const bool accountClicked = QMetaObject::invokeMethod(accountChip.get(), "click");
        QCoreApplication::processEvents();
        QEventLoop accountMenuTransition;
        QTimer::singleShot(120, &accountMenuTransition, &QEventLoop::quit);
        accountMenuTransition.exec();
        const bool standaloneAccountMenuOpened = accountMenu
            && accountMenu->property("opened").toBool();
        const auto findVisualItem = [](auto &&self, QQuickItem *parent, const QString &name) -> QQuickItem * {
            if (parent == nullptr)
                return nullptr;
            if (parent->objectName() == name)
                return parent;
            for (auto *child : parent->childItems()) {
                if (auto *match = self(self, child, name))
                    return match;
            }
            return nullptr;
        };
        auto *accountMenuItem = findVisualItem(findVisualItem, accountWindow.contentItem(), QStringLiteral("accountSwitcherMenuItem"));
        auto *accountEditButton = findVisualItem(findVisualItem, accountWindow.contentItem(), QStringLiteral("accountRenameButton"));
        const bool customAccountNameShown = accountMenuItem
            && accountMenuItem->property("text").toString() == QStringLiteral("Personal");
        const bool accountEditAvailable = accountEditButton && accountEditButton->isVisible()
            && accountEditButton->width() >= 40.0 && accountEditButton->height() >= 40.0;
        if (accountMenu)
            QMetaObject::invokeMethod(accountMenu, "close");
        QCoreApplication::processEvents();

        QQmlComponent chatComponent(&engine, QUrl(QStringLiteral("qrc:/qt/qml/org/whatsappgo/qml/ChatListDelegate.qml")));
        const QVariantMap chat{
            {QStringLiteral("jid"), QStringLiteral("123@s.whatsapp.net")},
            {QStringLiteral("title"), QStringLiteral("Unread chat")},
            {QStringLiteral("last_message_at"), QDateTime::currentMSecsSinceEpoch()},
            {QStringLiteral("last_message_preview"), QStringLiteral("First preview line\nSecond preview line\nThird preview line")},
            {QStringLiteral("unread_count"), 12},
        };
        std::unique_ptr<QObject> chatDelegate(chatComponent.createWithInitialProperties({
            {QStringLiteral("width"), 436},
            {QStringLiteral("modelData"), chat},
            {QStringLiteral("current"), false},
        }));
        if (!chatDelegate)
            return EXIT_FAILURE;
        QCoreApplication::processEvents();
        auto *timestamp = qobject_cast<QQuickItem *>(chatDelegate->findChild<QObject *>(QStringLiteral("chatTimestamp")));
        auto *badge = qobject_cast<QQuickItem *>(chatDelegate->findChild<QObject *>(QStringLiteral("unreadBadge")));
        auto *preview = qobject_cast<QQuickItem *>(chatDelegate->findChild<QObject *>(QStringLiteral("chatPreview")));
        auto *rowBackground = qobject_cast<QQuickItem *>(chatDelegate->findChild<QObject *>(QStringLiteral("chatRowBackground")));
        auto *chatItem = qobject_cast<QQuickItem *>(chatDelegate.get());
        if (!timestamp || !badge || !preview || !chatItem || !rowBackground)
            return EXIT_FAILURE;
        const auto timestampRight = timestamp->mapToItem(chatItem, QPointF(timestamp->width(), 0)).x();
        const auto badgeRight = badge->mapToItem(chatItem, QPointF(badge->width(), 0)).x();

        QQmlComponent messageComponent(&engine, QUrl(QStringLiteral("qrc:/qt/qml/org/whatsappgo/qml/MessageDelegate.qml")));
        const auto shortLine = QStringLiteral("This is one short quoted line.");
        const auto multiline = QStringList(10, shortLine).join(QLatin1Char('\n'));
        const QVariantMap message{
            {QStringLiteral("id"), QStringLiteral("layout-message")},
            {QStringLiteral("body"), multiline},
            {QStringLiteral("kind"), QStringLiteral("text")},
            {QStringLiteral("from_me"), true},
            {QStringLiteral("timestamp"), 0},
            {QStringLiteral("status"), QStringLiteral("read")},
        };
        std::unique_ptr<QObject> messageDelegate(messageComponent.createWithInitialProperties({
            {QStringLiteral("width"), 800},
            {QStringLiteral("modelData"), message},
        }));
        if (!messageDelegate)
            return EXIT_FAILURE;
        QCoreApplication::processEvents();
        auto *bubble = qobject_cast<QQuickItem *>(messageDelegate->findChild<QObject *>(QStringLiteral("messageBubble")));
        auto *tail = qobject_cast<QQuickItem *>(messageDelegate->findChild<QObject *>(QStringLiteral("messageTail")));
        // Name and preview sit on consecutive lines, so the gap between them is
        // small; the row is no taller than WhatsApp Web's.
        const auto titleBottom = timestamp->mapToItem(chatItem, QPointF(0, timestamp->height())).y();
        const auto previewTop = preview->mapToItem(chatItem, QPointF(0, 0)).y();
        qInfo().noquote() << QStringLiteral("chat row height=%1 name/preview gap=%2")
                                 .arg(chatItem->height()).arg(previewTop - titleBottom);
        auto *mainRoot = engine.rootObjects().isEmpty() ? nullptr : engine.rootObjects().constFirst();
        auto *chatViewport = mainRoot ? qobject_cast<QQuickItem *>(mainRoot->findChild<QObject *>(QStringLiteral("chatListViewport"))) : nullptr;
        auto *chatListItem = mainRoot ? qobject_cast<QQuickItem *>(mainRoot->findChild<QObject *>(QStringLiteral("chatList"))) : nullptr;
        auto *chatScrollBar = mainRoot ? qobject_cast<QQuickItem *>(mainRoot->findChild<QObject *>(QStringLiteral("chatListScrollBar"))) : nullptr;
        auto *actualAccountButton = mainRoot ? mainRoot->findChild<QObject *>(QStringLiteral("accountSwitcherButton")) : nullptr;
        const bool actualAccountClicked = actualAccountButton && QMetaObject::invokeMethod(actualAccountButton, "click");
        QCoreApplication::processEvents();
        return accountUnreadBadge && accountUnreadBadge->isVisible() && accountChipItem
                && accountClicked && standaloneAccountMenuOpened
                && customAccountNameShown
                && accountEditAvailable
                && accountMenu->property("height").toReal() >= 100.0
                && accountMenu->property("width").toReal() == 244.0
                && qFuzzyIsNull(accountMenu->property("x").toReal())
                && accountMenu->property("y").toReal() >= accountChip->property("height").toReal()
                && actualAccountClicked
                && accountUnreadBadge->width() >= 19.0 && accountChip->property("width").toReal() == 44.0
                && accountChip->property("totalUnread").toInt() == 12
                && qAbs(timestampRight - badgeRight) <= 2.0
                && chatItem->height() <= 76.0
                && chatItem->x() >= 8.0
                && rowBackground->property("radius").toReal() >= 10.0
                && chatViewport && chatListItem && chatScrollBar
                && chatListItem->width() + chatScrollBar->width() <= chatViewport->width()
                && chatScrollBar->width() <= 8.0
                && (previewTop - titleBottom) <= 8.0
                && preview->implicitHeight() <= 22.0
                && !preview->property("text").toString().contains(QStringLiteral("<br>"))
                && preview->property("text").toString().contains(QStringLiteral("line Second"))
                && bubble && bubble->width() < 360.0
                && tail && qFuzzyIsNull(tail->rotation()) && tail->z() >= 0.0
            ? EXIT_SUCCESS
            : EXIT_FAILURE;
    }
    if (messageLayoutTest) {
        QQmlComponent component(&engine, QUrl(QStringLiteral("qrc:/qt/qml/org/whatsappgo/qml/MessageDelegate.qml")));
        const qreal paneWidth = 1000;
        bool passed = true;
        const auto require = [&passed](bool condition, const QString &description) {
            if (!condition) {
                passed = false;
                qInfo().noquote() << QStringLiteral("FAIL: ") + description;
            }
        };

        // A long unbroken line must wrap inside a bubble that stays a fraction
        // of the conversation width instead of stretching across the pane.
        QVariantMap longMessage{
            {QStringLiteral("id"), QStringLiteral("long-line")},
            {QStringLiteral("body"), QStringLiteral("https://en.wikipedia.org/wiki/B4-4 %1").arg(QString(400, QLatin1Char('x')))},
            {QStringLiteral("kind"), QStringLiteral("text")},
            {QStringLiteral("from_me"), true},
            {QStringLiteral("timestamp"), 0},
            {QStringLiteral("status"), QStringLiteral("read")},
        };
        std::unique_ptr<QObject> longDelegate(component.createWithInitialProperties({
            {QStringLiteral("width"), paneWidth},
            {QStringLiteral("modelData"), longMessage},
        }));
        if (!longDelegate)
            return EXIT_FAILURE;
        QCoreApplication::processEvents();
        auto *longBubble = qobject_cast<QQuickItem *>(longDelegate->findChild<QObject *>(QStringLiteral("messageBubble")));
        auto *longBody = qobject_cast<QQuickItem *>(longDelegate->findChild<QObject *>(QStringLiteral("messageBody")));
        if (!longBubble || !longBody)
            return EXIT_FAILURE;
        qInfo().noquote() << QStringLiteral("long-line bubble width=%1 pane=%2 ratio=%3")
                                 .arg(longBubble->width()).arg(paneWidth).arg(longBubble->width() / paneWidth);
        require(longBubble->width() <= paneWidth * 0.70,
                QStringLiteral("bubble %1 exceeds 70%% of the %2 pane").arg(longBubble->width()).arg(paneWidth));
        require(longBubble->width() >= 200.0,
                QStringLiteral("bubble %1 collapsed below a readable width").arg(longBubble->width()));
        require(longDelegate->property("bodyFontSize").toInt() >= 15,
                QStringLiteral("message body text is smaller than 15 px"));
        require(longDelegate->property("horizontalPadding").toReal() >= 11.0,
                QStringLiteral("message bubble horizontal padding is below 11 px"));

        const auto bodyLeft = longBody->mapToItem(longBubble, QPointF(0, 0)).x();
        const auto bodyRight = longBody->mapToItem(longBubble, QPointF(longBody->width(), 0)).x();
        qInfo().noquote() << QStringLiteral("body left=%1 right=%2 bubble=%3").arg(bodyLeft).arg(bodyRight).arg(longBubble->width());
        require(bodyLeft >= 8.0, QStringLiteral("left padding %1 is below 8").arg(bodyLeft));
        require(longBubble->width() - bodyRight >= 8.0,
                QStringLiteral("right padding %1 is below 8").arg(longBubble->width() - bodyRight));

        // Right-to-left text must keep the same padding on the side it starts from.
        QVariantMap rtlMessage = longMessage;
        rtlMessage[QStringLiteral("id")] = QStringLiteral("rtl-line");
        rtlMessage[QStringLiteral("body")] = QStringLiteral("אני ממש שוקל לפטר אותה בעקבות הסרטון שראיתי אתמול בערב על כל הנושא הזה");
        std::unique_ptr<QObject> rtlDelegate(component.createWithInitialProperties({
            {QStringLiteral("width"), paneWidth},
            {QStringLiteral("modelData"), rtlMessage},
        }));
        if (!rtlDelegate)
            return EXIT_FAILURE;
        QCoreApplication::processEvents();
        auto *rtlBubble = qobject_cast<QQuickItem *>(rtlDelegate->findChild<QObject *>(QStringLiteral("messageBubble")));
        auto *rtlBody = qobject_cast<QQuickItem *>(rtlDelegate->findChild<QObject *>(QStringLiteral("messageBody")));
        if (!rtlBubble || !rtlBody)
            return EXIT_FAILURE;
        const auto rtlRight = rtlBody->mapToItem(rtlBubble, QPointF(rtlBody->width(), 0)).x();
        qInfo().noquote() << QStringLiteral("rtl bubble width=%1 body right=%2").arg(rtlBubble->width()).arg(rtlRight);
        require(rtlBubble->width() <= paneWidth * 0.70,
                QStringLiteral("RTL bubble %1 exceeds 70%% of the pane").arg(rtlBubble->width()));
        require(rtlBubble->width() - rtlRight >= 8.0,
                QStringLiteral("RTL right padding %1 is below 8").arg(rtlBubble->width() - rtlRight));

        // Bubbles must hug their text. Emoji, links, and quoted replies are
        // the shapes that previously stretched to the full conversation width.
        struct Shape {
            const char *name;
            QVariantMap message;
        };
        const QList<Shape> shapes{
            {"emoji", {{QStringLiteral("id"), QStringLiteral("s1")}, {QStringLiteral("kind"), QStringLiteral("text")},
                       {QStringLiteral("body"), QStringLiteral("\U0001F60A")}}},
            {"short", {{QStringLiteral("id"), QStringLiteral("s2")}, {QStringLiteral("kind"), QStringLiteral("text")},
                       {QStringLiteral("body"), QStringLiteral("ok")}}},
            {"link", {{QStringLiteral("id"), QStringLiteral("s3")}, {QStringLiteral("kind"), QStringLiteral("text")},
                      {QStringLiteral("body"), QStringLiteral("https://www.youtube.com/watch?v=5DdABvSTA0I")}}},
            {"reply", {{QStringLiteral("id"), QStringLiteral("s4")}, {QStringLiteral("kind"), QStringLiteral("text")},
                       {QStringLiteral("body"), QStringLiteral("yes")},
                       {QStringLiteral("reply_to"), QStringLiteral("s3")},
                       {QStringLiteral("reply_preview"), QStringLiteral("https://www.youtube.com/watch?v=5DdABvSTA0I")}}},
            {"reaction", {{QStringLiteral("id"), QStringLiteral("s5")}, {QStringLiteral("kind"), QStringLiteral("text")},
                          {QStringLiteral("body"), QStringLiteral("hi")},
                          {QStringLiteral("reactions"), QVariantList{QVariantMap{{QStringLiteral("emoji"), QStringLiteral("\U0001F44D")}}}}}},
        };
        for (const auto &shape : shapes) {
            auto payload = shape.message;
            payload.insert(QStringLiteral("timestamp"), 0);
            payload.insert(QStringLiteral("status"), QStringLiteral("received"));
            payload.insert(QStringLiteral("from_me"), false);
            std::unique_ptr<QObject> shapeDelegate(component.createWithInitialProperties({
                {QStringLiteral("width"), paneWidth},
                {QStringLiteral("modelData"), payload},
            }));
            if (!shapeDelegate)
                return EXIT_FAILURE;
            QCoreApplication::processEvents();
            auto *shapeBubble = qobject_cast<QQuickItem *>(shapeDelegate->findChild<QObject *>(QStringLiteral("messageBubble")));
            if (shapeBubble == nullptr)
                return EXIT_FAILURE;
            qInfo().noquote() << QStringLiteral("shape %1 bubble=%2").arg(QString::fromLatin1(shape.name)).arg(shapeBubble->width());
            require(shapeBubble->width() <= 460.0,
                    QStringLiteral("%1 bubble is %2 px wide; it should hug its text")
                        .arg(QString::fromLatin1(shape.name)).arg(shapeBubble->width()));
        }

        // A delegate is reused for messages from either side. Rebinding it must
        // not leave the bubble stretched across the conversation.
        QVariantMap outgoing{
            {QStringLiteral("id"), QStringLiteral("reuse-1")}, {QStringLiteral("kind"), QStringLiteral("text")},
            {QStringLiteral("body"), QStringLiteral("a message that is long enough to need most of the bubble width")},
            {QStringLiteral("from_me"), true}, {QStringLiteral("timestamp"), 0}, {QStringLiteral("status"), QStringLiteral("read")},
        };
        std::unique_ptr<QObject> reused(component.createWithInitialProperties({
            {QStringLiteral("width"), paneWidth},
            {QStringLiteral("modelData"), outgoing},
        }));
        if (!reused)
            return EXIT_FAILURE;
        QCoreApplication::processEvents();
        auto *reusedBubble = qobject_cast<QQuickItem *>(reused->findChild<QObject *>(QStringLiteral("messageBubble")));
        if (reusedBubble == nullptr)
            return EXIT_FAILURE;
        const auto outgoingWidth = reusedBubble->width();
        for (int round = 0; round < 4; ++round) {
            QVariantMap rebound{
                {QStringLiteral("id"), QStringLiteral("reuse-%1").arg(round + 2)},
                {QStringLiteral("kind"), QStringLiteral("text")},
                {QStringLiteral("body"), QStringLiteral("ok")},
                {QStringLiteral("from_me"), round % 2 == 0},
                {QStringLiteral("timestamp"), 0},
                {QStringLiteral("status"), QStringLiteral("received")},
            };
            reused->setProperty("modelData", rebound);
            QCoreApplication::processEvents();
            qInfo().noquote() << QStringLiteral("reuse round %1 from_me=%2 bubble=%3")
                                     .arg(round).arg(round % 2 == 0).arg(reusedBubble->width());
            require(reusedBubble->width() <= 120.0,
                    QStringLiteral("after reuse the bubble is %1 px wide for a two-letter message")
                        .arg(reusedBubble->width()));
        }
        Q_UNUSED(outgoingWidth);

        // A photo that has only its inline thumbnail must still be shown,
        // rather than reduced to a download row.
        QImage thumbnail(64, 48, QImage::Format_ARGB32_Premultiplied);
        thumbnail.fill(QColor(QStringLiteral("#25D366")));
        const auto thumbnailPath = QDir(QDir::tempPath()).filePath(QStringLiteral("whatsappgo-layout-thumb.jpg"));
        if (!thumbnail.save(thumbnailPath, "JPG"))
            return EXIT_FAILURE;
        const QVariantMap photo{
            {QStringLiteral("id"), QStringLiteral("photo-1")},
            {QStringLiteral("kind"), QStringLiteral("image")},
            {QStringLiteral("body"), QStringLiteral("caption")},
            {QStringLiteral("media_thumbnail"), thumbnailPath},
            {QStringLiteral("media_mime"), QStringLiteral("image/jpeg")},
            {QStringLiteral("from_me"), true},
            {QStringLiteral("timestamp"), 0},
            {QStringLiteral("status"), QStringLiteral("read")},
        };
        std::unique_ptr<QObject> photoDelegate(component.createWithInitialProperties({
            {QStringLiteral("width"), paneWidth},
            {QStringLiteral("modelData"), photo},
        }));
        if (!photoDelegate)
            return EXIT_FAILURE;
        QCoreApplication::processEvents();
        auto *photoImage = qobject_cast<QQuickItem *>(photoDelegate->findChild<QObject *>(QStringLiteral("messageMedia")));
        auto *photoBubble = qobject_cast<QQuickItem *>(photoDelegate->findChild<QObject *>(QStringLiteral("messageBubble")));
        require(photoImage != nullptr && photoImage->isVisible(),
                QStringLiteral("photo thumbnail is not displayed"));
        require(photoBubble != nullptr && photoBubble->width() <= paneWidth * 0.70,
                QStringLiteral("photo bubble is wider than 70%% of the pane"));

        // A video with a thumbnail shows the same preview plus a play control.
        QVariantMap video = photo;
        video[QStringLiteral("id")] = QStringLiteral("video-1");
        video[QStringLiteral("kind")] = QStringLiteral("video");
        video[QStringLiteral("body")] = QString();
        std::unique_ptr<QObject> videoDelegate(component.createWithInitialProperties({
            {QStringLiteral("width"), paneWidth},
            {QStringLiteral("modelData"), video},
        }));
        if (!videoDelegate)
            return EXIT_FAILURE;
        QCoreApplication::processEvents();
        auto *videoImage = qobject_cast<QQuickItem *>(videoDelegate->findChild<QObject *>(QStringLiteral("messageMedia")));
        auto *playBadge = qobject_cast<QQuickItem *>(videoDelegate->findChild<QObject *>(QStringLiteral("mediaPlayBadge")));
        require(videoImage != nullptr && videoImage->isVisible(),
                QStringLiteral("video thumbnail is not displayed"));
        require(playBadge != nullptr && playBadge->isVisible(),
                QStringLiteral("video play badge is missing"));

        // Playback is a lazy singleton. A video must wait until Main's overlay
        // has supplied its VideoOutput; starting the decoder against a null
        // surface produces audio with a permanently blank video pane.
        QQmlEngine isolatedPlaybackEngine;
        auto *playback = isolatedPlaybackEngine.singletonInstance<QObject *>(
            QStringLiteral("org.whatsappgo"), QStringLiteral("Playback"));
        require(playback != nullptr, QStringLiteral("video playback singleton is unavailable"));
        if (playback != nullptr) {
            const bool started = QMetaObject::invokeMethod(
                playback, "start",
                Q_ARG(QVariant, QVariant(QStringLiteral("layout-video"))),
                Q_ARG(QVariant, QVariant(QStringLiteral("/tmp/not-opened-before-surface.mp4"))),
                Q_ARG(QVariant, QVariant(true)));
            require(started, QStringLiteral("video playback start function is unavailable"));
            require(playback->property("waitingForVideoSurface").toBool(),
                    QStringLiteral("video playback started before its display surface existed"));
            QMetaObject::invokeMethod(playback, "stop");
        }

        // The other person's name belongs above group messages only.
        for (const auto &chatJid : {QStringLiteral("123@s.whatsapp.net"), QStringLiteral("456@g.us")}) {
            const bool group = chatJid.endsWith(QStringLiteral("@g.us"));
            const QVariantMap incoming{
                {QStringLiteral("id"), QStringLiteral("sender-%1").arg(chatJid)},
                {QStringLiteral("chat_jid"), chatJid},
                {QStringLiteral("kind"), QStringLiteral("text")},
                {QStringLiteral("body"), QStringLiteral("hello")},
                {QStringLiteral("sender_name"), QStringLiteral("Zone Yalo")},
                {QStringLiteral("from_me"), false},
                {QStringLiteral("timestamp"), 0},
                {QStringLiteral("status"), QStringLiteral("received")},
            };
            std::unique_ptr<QObject> senderDelegate(component.createWithInitialProperties({
                {QStringLiteral("width"), paneWidth},
                {QStringLiteral("modelData"), incoming},
            }));
            if (!senderDelegate)
                return EXIT_FAILURE;
            QCoreApplication::processEvents();
            bool shown = false;
            for (auto *label : senderDelegate->findChildren<QQuickItem *>()) {
                if (label->property("text").toString() == QStringLiteral("Zone Yalo") && label->isVisible())
                    shown = true;
            }
            qInfo().noquote() << QStringLiteral("sender name in %1 shown=%2").arg(chatJid).arg(shown);
            require(shown == group,
                    group ? QStringLiteral("a group message does not name its sender")
                          : QStringLiteral("a one-to-one message repeats the other person's name"));
        }

        // A shared contact and a shared place get cards, not bare rows.
        const QVariantMap sharedContact{
            {QStringLiteral("id"), QStringLiteral("contact-1")},
            {QStringLiteral("kind"), QStringLiteral("contact")},
            {QStringLiteral("body"), QStringLiteral("Adony Robles")},
            {QStringLiteral("contact_name"), QStringLiteral("Adony Robles")},
            {QStringLiteral("contact_phone"), QStringLiteral("+57 311 2522689")},
            {QStringLiteral("contact_count"), 1},
            {QStringLiteral("from_me"), false},
            {QStringLiteral("timestamp"), 0},
            {QStringLiteral("status"), QStringLiteral("received")},
        };
        std::unique_ptr<QObject> contactDelegate(component.createWithInitialProperties({
            {QStringLiteral("width"), paneWidth},
            {QStringLiteral("modelData"), sharedContact},
        }));
        if (!contactDelegate)
            return EXIT_FAILURE;
        QCoreApplication::processEvents();
        auto *sharedCard = qobject_cast<QQuickItem *>(contactDelegate->findChild<QObject *>(QStringLiteral("contactCard")));
        auto *sharedCardAction = qobject_cast<QQuickItem *>(contactDelegate->findChild<QObject *>(QStringLiteral("contactAction")));
        auto *contactFallback = qobject_cast<QQuickItem *>(contactDelegate->findChild<QObject *>(QStringLiteral("mediaAction")));
        require(sharedCard != nullptr && sharedCard->isVisible() && sharedCard->height() > 40.0,
                QStringLiteral("shared contact has no card"));
        require(sharedCardAction != nullptr && sharedCardAction->isVisible(),
                QStringLiteral("shared contact offers no way to message them"));
        require(contactFallback == nullptr || !contactFallback->isVisible(),
                QStringLiteral("shared contact still shows the plain attachment row"));

        QImage mapThumb(120, 80, QImage::Format_ARGB32_Premultiplied);
        mapThumb.fill(QColor(QStringLiteral("#8FBF7F")));
        const auto mapPath = QDir(QDir::tempPath()).filePath(QStringLiteral("whatsappgo-map.jpg"));
        if (!mapThumb.save(mapPath, "JPG"))
            return EXIT_FAILURE;
        const QVariantMap sharedPlace{
            {QStringLiteral("id"), QStringLiteral("place-1")},
            {QStringLiteral("kind"), QStringLiteral("location")},
            {QStringLiteral("body"), QStringLiteral("Bogot\u00E1")},
            {QStringLiteral("latitude"), 4.60971},
            {QStringLiteral("longitude"), -74.08175},
            {QStringLiteral("media_thumbnail"), mapPath},
            {QStringLiteral("from_me"), false},
            {QStringLiteral("timestamp"), 0},
            {QStringLiteral("status"), QStringLiteral("received")},
        };
        std::unique_ptr<QObject> placeDelegate(component.createWithInitialProperties({
            {QStringLiteral("width"), paneWidth},
            {QStringLiteral("modelData"), sharedPlace},
        }));
        if (!placeDelegate)
            return EXIT_FAILURE;
        QCoreApplication::processEvents();
        auto *placeCard = qobject_cast<QQuickItem *>(placeDelegate->findChild<QObject *>(QStringLiteral("locationCard")));
        auto *placeMap = qobject_cast<QQuickItem *>(placeDelegate->findChild<QObject *>(QStringLiteral("locationMap")));
        require(placeCard != nullptr && placeCard->isVisible() && placeCard->height() > 100.0,
                QStringLiteral("shared place has no card"));
        require(placeMap != nullptr && placeMap->isVisible(), QStringLiteral("shared place shows no map"));
        QFile::remove(mapPath);

        // A short message keeps its timestamp on the same line.
        const QVariantMap shortMessage{
            {QStringLiteral("id"), QStringLiteral("short-1")},
            {QStringLiteral("kind"), QStringLiteral("text")},
            {QStringLiteral("body"), QStringLiteral("No")},
            {QStringLiteral("from_me"), false},
            {QStringLiteral("timestamp"), 0},
            {QStringLiteral("status"), QStringLiteral("received")},
        };
        std::unique_ptr<QObject> shortDelegate(component.createWithInitialProperties({
            {QStringLiteral("width"), paneWidth},
            {QStringLiteral("modelData"), shortMessage},
        }));
        if (!shortDelegate)
            return EXIT_FAILURE;
        QCoreApplication::processEvents();
        auto *shortBubble = qobject_cast<QQuickItem *>(shortDelegate->findChild<QObject *>(QStringLiteral("messageBubble")));
        auto *shortBody = qobject_cast<QQuickItem *>(shortDelegate->findChild<QObject *>(QStringLiteral("messageBody")));
        if (shortBubble != nullptr && shortBody != nullptr) {
            qInfo().noquote() << QStringLiteral("short message bubble=%1 height=%2 body height=%3")
                                     .arg(shortBubble->width()).arg(shortBubble->height()).arg(shortBody->height());
            // One line of text plus padding, not two lines.
            require(shortBubble->height() <= shortBody->height() + 26.0,
                    QStringLiteral("a short message spends a second line on its timestamp"));
        }

        // Delivery marks are one overlapping symbol, not two spaced ticks.
        struct ReceiptCase {
            const char *status;
            bool doubleTick;
            bool blue;
        };
        for (const auto &receiptCase : {ReceiptCase{"sent", false, false}, ReceiptCase{"delivered", true, false},
                                        ReceiptCase{"read", true, true}, ReceiptCase{"played", true, true}}) {
            const QVariantMap sent{
                {QStringLiteral("id"), QStringLiteral("receipt-%1").arg(QString::fromLatin1(receiptCase.status))},
                {QStringLiteral("kind"), QStringLiteral("text")},
                {QStringLiteral("body"), QStringLiteral("ok")},
                {QStringLiteral("from_me"), true},
                {QStringLiteral("timestamp"), 0},
                {QStringLiteral("status"), QString::fromLatin1(receiptCase.status)},
            };
            std::unique_ptr<QObject> receiptDelegate(component.createWithInitialProperties({
                {QStringLiteral("width"), paneWidth},
                {QStringLiteral("modelData"), sent},
            }));
            if (!receiptDelegate)
                return EXIT_FAILURE;
            QCoreApplication::processEvents();
            auto *mark = qobject_cast<QQuickItem *>(receiptDelegate->findChild<QObject *>(QStringLiteral("readReceipt")));
            if (mark == nullptr) {
                require(false, QStringLiteral("delivery mark is missing"));
                continue;
            }
            const auto width = mark->width();
            const auto seen = mark->property("seen").toBool();
            const auto separation = mark->property("tickSeparation").toReal();
            qInfo().noquote() << QStringLiteral("receipt %1 width=%2 seen=%3")
                                     .arg(QString::fromLatin1(receiptCase.status)).arg(width).arg(seen);
            // WhatsApp's double mark is a compact, overlapping symbol. Keeping
            // it at 13 px or less catches the visually separated "✓ ✓" shape
            // while still leaving the second stroke distinct at 1x scale.
            require(receiptCase.doubleTick ? (width > 11.0 && width <= 13.0) : qFuzzyCompare(width, 11.0),
                    QStringLiteral("%1 mark is %2 px wide").arg(QString::fromLatin1(receiptCase.status)).arg(width));
            require(receiptCase.doubleTick ? (separation > 0.0 && separation <= 3.0) : qFuzzyIsNull(separation),
                    QStringLiteral("%1 tick separation is %2 px")
                        .arg(QString::fromLatin1(receiptCase.status)).arg(separation));
            require(seen == receiptCase.blue,
                    QStringLiteral("%1 mark colour is wrong").arg(QString::fromLatin1(receiptCase.status)));
        }

        // A link preview shows the page the sender's client resolved.
        QImage linkThumb(120, 63, QImage::Format_ARGB32_Premultiplied);
        linkThumb.fill(QColor(QStringLiteral("#1DAA61")));
        const auto linkThumbPath = QDir(QDir::tempPath()).filePath(QStringLiteral("whatsappgo-link.jpg"));
        if (!linkThumb.save(linkThumbPath, "JPG"))
            return EXIT_FAILURE;
        const QVariantMap linkMessage{
            {QStringLiteral("id"), QStringLiteral("link-1")},
            {QStringLiteral("kind"), QStringLiteral("text")},
            {QStringLiteral("body"), QStringLiteral("https://www.youtube.com/watch?v=5DdABvSTA0I")},
            {QStringLiteral("link_url"), QStringLiteral("https://www.youtube.com/watch?v=5DdABvSTA0I")},
            {QStringLiteral("link_title"), QStringLiteral("A video title")},
            {QStringLiteral("link_description"), QStringLiteral("The description the sender's client resolved.")},
            {QStringLiteral("link_thumbnail"), linkThumbPath},
            {QStringLiteral("from_me"), false},
            {QStringLiteral("timestamp"), 0},
            {QStringLiteral("status"), QStringLiteral("received")},
        };
        std::unique_ptr<QObject> linkDelegate(component.createWithInitialProperties({
            {QStringLiteral("width"), paneWidth},
            {QStringLiteral("modelData"), linkMessage},
        }));
        if (!linkDelegate)
            return EXIT_FAILURE;
        QCoreApplication::processEvents();
        auto *card = qobject_cast<QQuickItem *>(linkDelegate->findChild<QObject *>(QStringLiteral("linkPreview")));
        auto *cardImage = qobject_cast<QQuickItem *>(linkDelegate->findChild<QObject *>(QStringLiteral("linkPreviewImage")));
        auto *linkHover = linkDelegate->findChild<QObject *>(QStringLiteral("messageLinkHover"));
        auto *linkBubble = qobject_cast<QQuickItem *>(linkDelegate->findChild<QObject *>(QStringLiteral("messageBubble")));
        require(card != nullptr && card->isVisible(), QStringLiteral("link preview card is missing"));
        require(cardImage != nullptr && cardImage->isVisible(), QStringLiteral("link preview picture is missing"));
        require(linkHover != nullptr, QStringLiteral("message links have no pointing-hand hover handler"));
        if (card != nullptr && linkBubble != nullptr) {
            qInfo().noquote() << QStringLiteral("link card width=%1 height=%2 bubble=%3")
                                     .arg(card->width()).arg(card->height()).arg(linkBubble->width());
            require(card->height() > 60.0, QStringLiteral("link preview card has no height"));
            require(linkBubble->width() <= paneWidth * 0.70, QStringLiteral("link bubble is too wide"));
        }
        QFile::remove(linkThumbPath);

        // Pasting a URL shows its resolved Open Graph card above the composer
        // before anything is sent.
        QQmlComponent composerPreviewComponent(&engine);
        composerPreviewComponent.setData("import QtQuick\nimport org.whatsappgo\nComposerLinkPreview {}",
                                         QUrl(QStringLiteral("composer-preview-test.qml")));
        std::unique_ptr<QObject> composerPreview(composerPreviewComponent.createWithInitialProperties({
            {QStringLiteral("width"), 720},
            {QStringLiteral("preview"), QVariantMap{
                {QStringLiteral("url"), QStringLiteral("https://yahoo.com")},
                {QStringLiteral("title"), QStringLiteral("Yahoo | Mail, Weather, Search")},
                {QStringLiteral("description"), QStringLiteral("Latest news coverage and more.")},
            }},
        }));
        if (!composerPreview) {
            qWarning().noquote() << composerPreviewComponent.errorString();
            require(false, QStringLiteral("composer link preview could not be created"));
        } else {
            QCoreApplication::processEvents();
            auto *previewItem = qobject_cast<QQuickItem *>(composerPreview.get());
            auto *previewTitle = composerPreview->findChild<QObject *>(QStringLiteral("composerLinkPreviewTitle"));
            require(previewItem != nullptr && previewItem->isVisible() && previewItem->height() >= 90.0,
                    QStringLiteral("pasted link has no composer OG card"));
            require(previewTitle != nullptr
                        && previewTitle->property("text").toString().startsWith(QStringLiteral("Yahoo")),
                    QStringLiteral("composer OG title is missing"));
        }

        // A voice note plays inside the window, so it shows a compact player
        // rather than a row that hands the file to the desktop.
        const QVariantMap voice{
            {QStringLiteral("id"), QStringLiteral("voice-1")},
            {QStringLiteral("kind"), QStringLiteral("audio")},
            {QStringLiteral("media_path"), QStringLiteral("/tmp/whatsappgo-voice.ogg")},
            {QStringLiteral("media_mime"), QStringLiteral("audio/ogg")},
            {QStringLiteral("media_duration"), 9},
            {QStringLiteral("audio_waveform"), QVariantList{10, 40, 90, 55, 20, 70, 35, 80}},
            {QStringLiteral("from_me"), false},
            {QStringLiteral("timestamp"), 0},
            {QStringLiteral("status"), QStringLiteral("received")},
        };
        std::unique_ptr<QObject> voiceDelegate(component.createWithInitialProperties({
            {QStringLiteral("width"), paneWidth},
            {QStringLiteral("modelData"), voice},
        }));
        if (!voiceDelegate)
            return EXIT_FAILURE;
        QCoreApplication::processEvents();
        auto *voiceRow = qobject_cast<QQuickItem *>(voiceDelegate->findChild<QObject *>(QStringLiteral("voiceRow")));
        auto *voicePlay = qobject_cast<QQuickItem *>(voiceDelegate->findChild<QObject *>(QStringLiteral("voicePlayButton")));
        auto *voiceOpen = qobject_cast<QQuickItem *>(voiceDelegate->findChild<QObject *>(QStringLiteral("mediaAction")));
        auto *voiceBubble = qobject_cast<QQuickItem *>(voiceDelegate->findChild<QObject *>(QStringLiteral("messageBubble")));
        require(voiceRow != nullptr && voiceRow->isVisible(), QStringLiteral("voice note has no player row"));
        require(voicePlay != nullptr && voicePlay->isVisible(), QStringLiteral("voice note has no play control"));
        require(voiceOpen == nullptr || !voiceOpen->isVisible(),
                QStringLiteral("voice note still offers the desktop open action"));
        auto *voiceProgress = qobject_cast<QQuickItem *>(voiceDelegate->findChild<QObject *>(QStringLiteral("voiceProgress")));
        auto *voiceAvatar = qobject_cast<QQuickItem *>(voiceDelegate->findChild<QObject *>(QStringLiteral("voiceAvatar")));
        auto *voiceDuration = voiceDelegate->findChild<QObject *>(QStringLiteral("voiceDuration"));
        require(voiceProgress != nullptr && voiceProgress->isVisible(), QStringLiteral("voice note has no waveform"));
        require(voiceAvatar != nullptr && voiceAvatar->isVisible() && voiceAvatar->width() >= 40.0,
                QStringLiteral("voice note has no sender avatar"));
        auto *waveformRow = qobject_cast<QQuickItem *>(voiceDelegate->findChild<QObject *>(QStringLiteral("voiceWaveform")));
        if (waveformRow != nullptr) {
            // One rounded bar per amplitude the sender recorded.
            const auto bars = waveformRow->childItems();
            int drawn = 0;
            for (auto *bar : bars) {
                if (bar->width() > 0 && bar->height() >= 3)
                    ++drawn;
            }
            qInfo().noquote() << QStringLiteral("waveform row %1x%2 children=%3 drawn=%4")
                                     .arg(waveformRow->width()).arg(waveformRow->height()).arg(bars.size()).arg(drawn);
            require(drawn >= 8, QStringLiteral("waveform drew %1 bars, expected one per amplitude").arg(drawn));
        } else {
            require(false, QStringLiteral("waveform row is missing"));
        }
        require(voiceDuration != nullptr && voiceDuration->property("visible").toBool()
                    && voiceDuration->property("text").toString() == QStringLiteral("0:09"),
                QStringLiteral("voice note does not show its length"));
        if (voiceRow != nullptr) {
            qInfo().noquote() << QStringLiteral("voice row height=%1").arg(voiceRow->height());
            require(voiceRow->height() <= 56.0,
                    QStringLiteral("voice row is %1 px tall, expected a compact control").arg(voiceRow->height()));
        }
        if (voiceBubble != nullptr) {
            require(voiceBubble->width() <= paneWidth * 0.70,
                    QStringLiteral("voice bubble %1 exceeds 70%% of the pane").arg(voiceBubble->width()));
        }

        // Without any local file a document still offers its download action.
        const QVariantMap document{
            {QStringLiteral("id"), QStringLiteral("doc-1")},
            {QStringLiteral("kind"), QStringLiteral("document")},
            {QStringLiteral("media_name"), QStringLiteral("report.pdf")},
            {QStringLiteral("from_me"), false},
            {QStringLiteral("timestamp"), 0},
            {QStringLiteral("status"), QStringLiteral("received")},
        };
        std::unique_ptr<QObject> documentDelegate(component.createWithInitialProperties({
            {QStringLiteral("width"), paneWidth},
            {QStringLiteral("modelData"), document},
        }));
        if (!documentDelegate)
            return EXIT_FAILURE;
        QCoreApplication::processEvents();
        auto *documentAction = qobject_cast<QQuickItem *>(documentDelegate->findChild<QObject *>(QStringLiteral("mediaAction")));
        require(documentAction != nullptr && documentAction->isVisible(),
                QStringLiteral("document download action is missing"));
        QFile::remove(thumbnailPath);
        return passed ? EXIT_SUCCESS : EXIT_FAILURE;
    }
    if (messageScrollTest) {
        auto *mainRoot = engine.rootObjects().isEmpty() ? nullptr : engine.rootObjects().constFirst();
        auto *messageList = mainRoot
            ? qobject_cast<QQuickItem *>(mainRoot->findChild<QObject *>(QStringLiteral("messageList")))
            : nullptr;
        auto *messages = qobject_cast<MessageListModel *>(backend.messages());
        if (messageList == nullptr || messages == nullptr)
            return EXIT_FAILURE;

        // This embedded test supplies its own model without opening a real
        // account chat. Reveal the conversation column so window-system wheel
        // events can actually hit the ListView rather than the empty-chat pane.
        auto *messageViewport = messageList->parentItem();
        auto *conversationColumn = messageViewport ? messageViewport->parentItem() : nullptr;
        if (conversationColumn != nullptr)
            conversationColumn->setVisible(true);
        QCoreApplication::processEvents();

        bool passed = true;
        const auto require = [&passed](bool condition, const QString &description) {
            if (!condition) {
                passed = false;
                qInfo().noquote() << QStringLiteral("FAIL: ") + description;
            }
        };
        const auto settle = [] {
            QEventLoop loop;
            QTimer::singleShot(80, &loop, &QEventLoop::quit);
            loop.exec();
        };
        QTemporaryDir scrollMediaDirectory;
        const auto scrollPreviewPath = scrollMediaDirectory.filePath(QStringLiteral("scroll-preview.jpg"));
        QImage scrollPreview(640, 360, QImage::Format_RGB32);
        scrollPreview.fill(QColor(QStringLiteral("#25D366")));
        require(scrollPreview.save(scrollPreviewPath),
                QStringLiteral("the mixed-height scroll fixture could not be written"));
        const auto makeMessage = [&scrollPreviewPath](int index) {
            QVariantMap result{
                {QStringLiteral("id"), QStringLiteral("scroll-%1").arg(index)},
                {QStringLiteral("kind"), QStringLiteral("text")},
                {QStringLiteral("body"), QStringLiteral("Message %1 with enough text to exercise row layout").arg(index)},
                {QStringLiteral("from_me"), index % 2 == 0},
                {QStringLiteral("timestamp"), index * 1000},
                {QStringLiteral("status"), QStringLiteral("read")},
            };
            if (index > 0 && index % 9 == 0) {
                result.insert(QStringLiteral("kind"), QStringLiteral("image"));
                result.insert(QStringLiteral("media_path"), scrollPreviewPath);
                result.insert(QStringLiteral("media_thumbnail"), scrollPreviewPath);
            } else if (index > 0 && index % 7 == 0) {
                result.insert(QStringLiteral("body"),
                              QStringLiteral("A deliberately taller message %1\nwith several lines\n"
                                             "so the wheel test crosses mixed delegate heights\n"
                                             "without changing its logical direction.").arg(index));
            }
            return result;
        };

        messageList->setProperty("initialPositionPending", true);
        messages->reset(QVariantList{makeMessage(0)});
        settle();
        require(messageList->property("contentY").toReal()
                    >= messageList->property("originY").toReal()
                        + messageList->property("contentHeight").toReal()
                        - messageList->height() - 48.0,
                QStringLiteral("a short conversation did not open at its tail"));
        require(messageList->property("topMargin").toReal() > messageList->height() / 2.0,
                QStringLiteral("a short conversation starts at the top instead of growing upward from the composer"));

        QVariantList history;
        for (int index = 0; index < 80; ++index)
            history.append(makeMessage(index));
        messageList->setProperty("initialPositionPending", true);
        messages->reset(history);
        settle();
        require(messageList->property("contentY").toReal()
                    >= messageList->property("originY").toReal()
                        + messageList->property("contentHeight").toReal()
                        - messageList->height() - 48.0,
                QStringLiteral("a full conversation did not open at its newest message"));

        // Exercise the same event path as a physical mouse wheel, delivered
        // directly to WhatsAppGo's QQuickWindow so desktop focus or another
        // application cannot affect the result.
        const qreal wheelTailY = messageList->property("contentY").toReal();
        const QPointF wheelPosition = messageList->mapToScene(
            QPointF(messageList->width() / 2.0, messageList->height() / 2.0));
        const QPointF globalWheelPosition = applicationWindow != nullptr
            ? QPointF(applicationWindow->mapToGlobal(wheelPosition.toPoint()))
            : wheelPosition;
        qreal previousWheelY = wheelTailY;
        bool wheelWasMonotonic = true;
        for (int tick = 0; tick < 16; ++tick) {
            if (applicationWindow != nullptr)
                qt_handleWheelEvent(applicationWindow, wheelPosition, globalWheelPosition,
                                    QPoint(), QPoint(0, 120), Qt::NoModifier,
                                    Qt::NoScrollPhase);
            QEventLoop wheelTickLoop;
            QTimer::singleShot(40, &wheelTickLoop, &QEventLoop::quit);
            wheelTickLoop.exec();
            const qreal currentWheelY = messageList->property("contentY").toReal();
            if (currentWheelY - 1.0 > previousWheelY)
                wheelWasMonotonic = false;
            previousWheelY = currentWheelY;
        }
        settle();
        require(wheelWasMonotonic,
                QStringLiteral("mixed-height rows made physical mouse-wheel scrolling jump backwards"));
        require(messageList->property("contentY").toReal() < wheelTailY - 400.0,
                QStringLiteral("physical mouse-wheel events stalled before reaching older messages"));

        QMetaObject::invokeMethod(messageList, "cancelFlick");
        messageList->setProperty("followTail", true);
        QMetaObject::invokeMethod(messageList, "scheduleTailPosition");
        settle();

        messageList->setProperty("followTail", true);
        messages->upsert(makeMessage(80));
        settle();
        require(messageList->property("contentY").toReal()
                    >= messageList->property("originY").toReal()
                        + messageList->property("contentHeight").toReal()
                        - messageList->height() - 48.0,
                QStringLiteral("an appended message moved a tail-following conversation away from the bottom"));

        const qreal tailY = messageList->property("contentY").toReal();
        messageList->setProperty("followTail", false);
        messageList->setProperty("contentY", tailY - 220.0);
        const qreal readingY = messageList->property("contentY").toReal();
        messages->upsert(makeMessage(81));
        settle();
        require(qAbs(messageList->property("contentY").toReal() - readingY) < 2.0,
                QStringLiteral("an appended message interrupted a reader browsing older history"));

        // A wheel gesture can begin in the same event-loop turn as an
        // appended row or a late media-size update. Reader movement must win
        // over that already queued tail position instead of being snapped
        // back to the newest message.
        messageList->setProperty("followTail", true);
        QMetaObject::invokeMethod(messageList, "scheduleTailPosition");
        settle();
        const qreal gestureTailY = messageList->property("contentY").toReal();
        messages->upsert(makeMessage(82));
        const bool movementStarted = QMetaObject::invokeMethod(messageList, "movementStarted");
        messageList->setProperty("contentY", gestureTailY - 220.0);
        const qreal gestureReadingY = messageList->property("contentY").toReal();
        settle();
        require(movementStarted,
                QStringLiteral("the message list did not expose its movement-start signal"));
        require(!messageList->property("followTail").toBool(),
                QStringLiteral("starting reader movement did not release the tail anchor"));
        require(qAbs(messageList->property("contentY").toReal() - gestureReadingY) < 2.0,
                QStringLiteral("a queued tail update snapped an active reader gesture back to the bottom"));

        // The timer marks its one-frame tail correction as positioningTail.
        // A wheel event can land during that exact frame; it must still cancel
        // tail following instead of being undone by the queued callLater.
        messageList->setProperty("followTail", true);
        messageList->setProperty("positioningTail", true);
        const bool releasedDuringPositioning = QMetaObject::invokeMethod(messageList, "releaseTail");
        require(releasedDuringPositioning,
                QStringLiteral("the message list did not expose its tail-release function"));
        require(!messageList->property("followTail").toBool(),
                QStringLiteral("a wheel gesture during tail positioning did not release the tail anchor"));
        messageList->setProperty("positioningTail", false);

        // Avatar/title refreshes emit selectedChatChanged for the chat that is
        // already open. They must not arm another initial tail jump, otherwise
        // the next incoming row snaps a reader out of older history.
        messages->clear();
        const bool preparedChat = QMetaObject::invokeMethod(
            messageList, "prepareForChat", Q_ARG(QVariant, QVariant(QStringLiteral("stable@lid"))));
        require(preparedChat, QStringLiteral("the message list did not expose its chat-position function"));
        messageList->setProperty("initialPositionPending", false);
        QMetaObject::invokeMethod(
            messageList, "prepareForChat", Q_ARG(QVariant, QVariant(QStringLiteral("stable@lid"))));
        require(!messageList->property("initialPositionPending").toBool(),
                QStringLiteral("refreshing metadata for the open chat re-armed its initial tail jump"));

        // Entering the top threshold must request another local history page
        // without installing a WheelHandler in front of ListView. The latter
        // consumes physical mouse-wheel ticks on Qt 6.8 even when configured
        // with blocking=false.
        auto *olderMessagesTimer = messageList->findChild<QObject *>(QStringLiteral("olderMessagesTimer"));
        messages->reset(history);
        settle();
        if (olderMessagesTimer != nullptr)
            QMetaObject::invokeMethod(olderMessagesTimer, "stop");
        messageList->setProperty("positioningTail", false);
        const qreal historyStart = messageList->property("originY").toReal();
        messageList->setProperty("contentY", historyStart + 200.0);
        messageList->setProperty("contentY", historyStart + 40.0);
        require(olderMessagesTimer != nullptr && olderMessagesTimer->property("running").toBool(),
                QStringLiteral("entering the top boundary did not request older messages"));
        if (olderMessagesTimer != nullptr)
            QMetaObject::invokeMethod(olderMessagesTimer, "stop");

        // Returning to a chat in the same session must paint its last local
        // page synchronously instead of showing an empty conversation while
        // another RPC round trip completes.
        backend.openChat(QStringLiteral("cache-a@s.whatsapp.net"), QStringLiteral("Cache A"));
        messages->reset(QVariantList{makeMessage(90)});
        backend.openChat(QStringLiteral("cache-b@s.whatsapp.net"), QStringLiteral("Cache B"));
        backend.openChat(QStringLiteral("cache-a@s.whatsapp.net"), QStringLiteral("Cache A"));
        require(messages->count() == 1
                    && messages->at(0).value(QStringLiteral("id")).toString() == QStringLiteral("scroll-90"),
                QStringLiteral("returning to a viewed chat did not restore its messages immediately"));

        return passed ? EXIT_SUCCESS : EXIT_FAILURE;
    }
    if (mediaPreviewTest) {
        if (engine.rootObjects().isEmpty())
            return EXIT_FAILURE;
        // Match the dimensions of the image that exposed the software-layer
        // crop without depending on a user's media cache.
        QImage source(1204, 1091, QImage::Format_ARGB32_Premultiplied);
        source.fill(QColor(QStringLiteral("#25D366")));
        QGuiApplication::clipboard()->setImage(source);
        auto *root = engine.rootObjects().constFirst();
        root->setProperty("width", 1760);
        root->setProperty("height", 1008);
        QCoreApplication::processEvents();

        // The paperclip opens the complete attachment chooser. Keep every
        // WhatsApp-style destination visible even when the linked-device API
        // cannot send that type yet, so unavailable choices can explain
        // themselves instead of disappearing.
        auto *attachmentButton = root->findChild<QObject *>(QStringLiteral("attachmentButton"));
        auto *attachmentMenu = root->findChild<QObject *>(QStringLiteral("attachmentMenu"));
        if (!attachmentButton || !attachmentMenu) {
            qWarning() << "attachment chooser missing" << attachmentButton << attachmentMenu;
            return EXIT_FAILURE;
        }
        if (!QMetaObject::invokeMethod(root, "toggleAttachmentMenu")) {
            qWarning() << "attachment menu could not be toggled";
            return EXIT_FAILURE;
        }
        QCoreApplication::processEvents();
        if (!attachmentMenu->property("visible").toBool()) {
            qWarning() << "attachment chooser did not become visible";
            return EXIT_FAILURE;
        }
        const QStringList attachmentActions{
            QStringLiteral("attachmentDocument"),
            QStringLiteral("attachmentPhotosVideos"),
            QStringLiteral("attachmentCamera"),
            QStringLiteral("attachmentAudio"),
            QStringLiteral("attachmentContact"),
            QStringLiteral("attachmentPoll"),
            QStringLiteral("attachmentEvent"),
            QStringLiteral("attachmentSticker"),
        };
        for (const auto &name : attachmentActions) {
            auto *action = qobject_cast<QQuickItem *>(root->findChild<QObject *>(name));
            if (!action || !action->isVisible() || action->height() < 44) {
                qWarning() << "attachment action invalid" << name << action
                           << (action ? action->isVisible() : false)
                           << (action ? action->height() : 0);
                return EXIT_FAILURE;
            }
        }
        QMetaObject::invokeMethod(attachmentMenu, "close");

        const bool invoked = QMetaObject::invokeMethod(root, "prepareClipboardPaste");
        QCoreApplication::processEvents();
        auto *preview = root->findChild<QObject *>(QStringLiteral("mediaPreviewOverlay"));
        if (!invoked || !preview || !preview->property("previewActive").toBool())
            return EXIT_FAILURE;

        const auto receivedImagePath = QDir(QDir::tempPath()).filePath(
            QStringLiteral("whatsappgo-native-viewer-test.jpg"));
        if (!source.save(receivedImagePath, "JPG"))
            return EXIT_FAILURE;
        QImage thumbnail(40, 40, QImage::Format_ARGB32_Premultiplied);
        thumbnail.fill(QColor(QStringLiteral("#EA0038")));
        const auto thumbnailPath = QDir(QDir::tempPath()).filePath(
            QStringLiteral("whatsappgo-native-viewer-thumbnail.jpg"));
        if (!thumbnail.save(thumbnailPath, "JPG"))
            return EXIT_FAILURE;
        const QVariant imageMessage = QVariantMap{
            {QStringLiteral("id"), QStringLiteral("received-image-test")},
            {QStringLiteral("kind"), QStringLiteral("image")},
            {QStringLiteral("media_thumbnail"), thumbnailPath},
            {QStringLiteral("body"), QStringLiteral("A received photo")},
            {QStringLiteral("from_me"), false},
            {QStringLiteral("timestamp"), QDateTime::currentMSecsSinceEpoch()},
        };
        const bool nativeViewerInvoked = QMetaObject::invokeMethod(
            root, "openChatImage", Q_ARG(QVariant, imageMessage));
        QCoreApplication::processEvents();
        auto *nativeViewer = root->findChild<QObject *>(QStringLiteral("chatMediaViewer"));
        auto *nativeImage = nativeViewer
            ? nativeViewer->findChild<QObject *>(QStringLiteral("chatMediaViewerImage")) : nullptr;
        auto *nativeClose = nativeViewer
            ? qobject_cast<QQuickItem *>(nativeViewer->findChild<QObject *>(QStringLiteral("chatMediaViewerCloseButton"))) : nullptr;
        const bool nativeViewerReady = nativeViewerInvoked && nativeViewer
            && nativeViewer->property("previewActive").toBool()
            && nativeImage && nativeImage->property("source").toUrl().isLocalFile()
            && nativeClose && nativeClose->width() >= 44 && nativeClose->height() >= 44;
        const bool startedWithThumbnail = nativeImage
            && nativeImage->property("source").toUrl().toLocalFile() == thumbnailPath;
        const bool completedDownload = QMetaObject::invokeMethod(
            root, "completeChatImageDownload",
            Q_ARG(QVariant, QStringLiteral("received-image-test")),
            Q_ARG(QVariant, receivedImagePath));
        QCoreApplication::processEvents();
        QEventLoop imageDecodeLoop;
        QTimer::singleShot(150, &imageDecodeLoop, &QEventLoop::quit);
        imageDecodeLoop.exec();
        const bool upgradedToFullImage = nativeImage
            && nativeImage->property("source").toUrl().toLocalFile() == receivedImagePath;
        auto *zoomIn = nativeViewer
            ? qobject_cast<QQuickItem *>(nativeViewer->findChild<QObject *>(QStringLiteral("chatMediaViewerZoomIn"))) : nullptr;
        auto *zoomOut = nativeViewer
            ? qobject_cast<QQuickItem *>(nativeViewer->findChild<QObject *>(QStringLiteral("chatMediaViewerZoomOut"))) : nullptr;
        auto *zoomWheel = nativeViewer
            ? nativeViewer->findChild<QObject *>(QStringLiteral("chatMediaViewerZoomWheel")) : nullptr;
        auto *zoomSurface = nativeViewer
            ? nativeViewer->findChild<QObject *>(QStringLiteral("chatMediaViewerZoomSurface")) : nullptr;
        auto *stageItem = zoomSurface ? qobject_cast<QQuickItem *>(zoomSurface)->parentItem() : nullptr;
        auto *viewerWindow = qobject_cast<QQuickWindow *>(root);
        const auto renderedViewer = viewerWindow ? viewerWindow->grabWindow() : QImage();
        const auto bottomProbe = stageItem
            ? stageItem->mapToScene(QPointF(stageItem->width() / 2, stageItem->height() - 8)) : QPointF();
        const auto bottomColor = renderedViewer.isNull()
            ? QColor() : renderedViewer.pixelColor(bottomProbe.toPoint());
        const auto paintedWidth = nativeImage ? nativeImage->property("paintedWidth").toReal() : 0.0;
        const auto paintedHeight = nativeImage ? nativeImage->property("paintedHeight").toReal() : 0.0;
        const bool fullImageFitsAtMinimumZoom = stageItem && nativeImage
            && nativeImage->property("sourceSize").toSize() == source.size()
            && paintedWidth > 0.0 && paintedHeight > 0.0
            && paintedWidth <= stageItem->width() + 0.5
            && paintedHeight <= stageItem->height() + 0.5
            && bottomColor.isValid() && bottomColor.green() > bottomColor.red()
            && bottomColor.green() > bottomColor.blue();
        const auto initialZoom = nativeViewer ? nativeViewer->property("zoomFactor").toReal() : 0.0;
        const bool zoomedIn = nativeViewer && QMetaObject::invokeMethod(
            nativeViewer, "adjustZoomFromWheel",
            Q_ARG(QVariant, 120.0), Q_ARG(QVariant, 160.0), Q_ARG(QVariant, 120.0));
        QCoreApplication::processEvents();
        const auto enlargedZoom = nativeViewer ? nativeViewer->property("zoomFactor").toReal() : 0.0;
        const auto anchoredPanX = nativeViewer ? nativeViewer->property("panX").toReal() : 0.0;
        const auto anchoredPanY = nativeViewer ? nativeViewer->property("panY").toReal() : 0.0;
        const bool zoomedOut = nativeViewer && QMetaObject::invokeMethod(
            nativeViewer, "adjustZoomFromWheel",
            Q_ARG(QVariant, -120.0), Q_ARG(QVariant, 160.0), Q_ARG(QVariant, 120.0));
        QCoreApplication::processEvents();
        const auto restoredZoom = nativeViewer ? nativeViewer->property("zoomFactor").toReal() : 0.0;
        const bool nativeZoomReady = zoomIn && zoomOut && zoomWheel && zoomSurface
            && zoomIn->width() >= 44 && zoomIn->height() >= 44
            && zoomOut->width() >= 44 && zoomOut->height() >= 44
            // Qt's software scene graph can crop a transformed cached layer
            // and clear the rest to black. The photo itself is cheap to
            // transform and must stay live so every edge remains available.
            && !zoomSurface->property("renderCached").toBool()
            && fullImageFitsAtMinimumZoom
            && qFuzzyCompare(initialZoom, 1.0) && zoomedIn
            && enlargedZoom > 1.15 && enlargedZoom < 1.25
            && (!qFuzzyIsNull(anchoredPanX) || !qFuzzyIsNull(anchoredPanY))
            && zoomedOut && qAbs(restoredZoom - 1.0) < 0.01
            && qFuzzyIsNull(nativeViewer->property("panX").toReal())
            && qFuzzyIsNull(nativeViewer->property("panY").toReal());
        const bool nativeViewerClosed = nativeClose && QMetaObject::invokeMethod(nativeClose, "click");
        QCoreApplication::processEvents();
        QFile::remove(receivedImagePath);
        QFile::remove(thumbnailPath);
        if (!nativeViewerReady || !startedWithThumbnail || !completedDownload || !upgradedToFullImage
                || !nativeZoomReady || !nativeViewerClosed
                || nativeViewer->property("previewActive").toBool())
            return EXIT_FAILURE;
        if (screenshotPath.isEmpty())
            return EXIT_SUCCESS;
    }
    if (chatFilterTest) {
        QQmlComponent component(&engine, QUrl(QStringLiteral("qrc:/qt/qml/org/whatsappgo/qml/ChatFilterBar.qml")));
        std::unique_ptr<QObject> bar(component.createWithInitialProperties({
            {QStringLiteral("width"), 436},
            {QStringLiteral("unreadCount"), 3},
        }));
        if (!bar)
            return EXIT_FAILURE;
        QCoreApplication::processEvents();
        auto *all = bar->findChild<QObject *>(QStringLiteral("filterAllButton"));
        auto *unread = bar->findChild<QObject *>(QStringLiteral("filterUnreadButton"));
        auto *favorites = bar->findChild<QObject *>(QStringLiteral("filterFavoritesButton"));
        auto *groups = bar->findChild<QObject *>(QStringLiteral("filterGroupsButton"));
        if (!all || !unread || !favorites || !groups)
            return EXIT_FAILURE;
        const bool clicked = QMetaObject::invokeMethod(unread, "click");
        QCoreApplication::processEvents();
        const bool unreadSelected = bar->property("selectedFilter").toString() == QStringLiteral("unread");
        const auto accepts = [&bar](const QVariantMap &chat) {
            QVariant accepted;
            return QMetaObject::invokeMethod(bar.get(), "accepts",
                       Q_RETURN_ARG(QVariant, accepted), Q_ARG(QVariant, chat))
                && accepted.toBool();
        };
        const bool unreadAccepted = accepts({{QStringLiteral("unread_count"), 2}})
            && !accepts({{QStringLiteral("unread_count"), 0}});
        bar->setProperty("selectedFilter", QStringLiteral("favorites"));
        const bool favoriteAccepted = accepts({{QStringLiteral("favorite"), true}})
            && !accepts({{QStringLiteral("favorite"), false}});
        bar->setProperty("selectedFilter", QStringLiteral("groups"));
        const bool groupAccepted = accepts({{QStringLiteral("is_group"), true}})
            && !accepts({{QStringLiteral("is_group"), false}});
        return all->property("text").toString() == QStringLiteral("All")
                && unread->property("text").toString() == QStringLiteral("Unread 3")
                && favorites->property("text").toString() == QStringLiteral("Favorites")
                && groups->property("text").toString() == QStringLiteral("Groups")
                && clicked && unreadSelected && unreadAccepted && favoriteAccepted && groupAccepted
            ? EXIT_SUCCESS
            : EXIT_FAILURE;
    }
    if (backendLifecycleTest) {
        auto verifyOwnership = [&backend] {
            if (backend.daemonConnected() && backend.findChild<QProcess *>() != nullptr)
                QCoreApplication::exit(EXIT_SUCCESS);
        };
        QObject::connect(&backend, &RpcClient::daemonConnectedChanged, &app, verifyOwnership);
        QTimer::singleShot(0, &app, verifyOwnership);
        QTimer::singleShot(6000, &app, [] { QCoreApplication::exit(EXIT_FAILURE); });
        return app.exec();
    }
    if (clipboardImageTest) {
        QImage source(3, 2, QImage::Format_ARGB32_Premultiplied);
        source.fill(QColor(QStringLiteral("#25D366")));
        QGuiApplication::clipboard()->setImage(source);
        if (!backend.clipboardHasImage())
            return EXIT_FAILURE;
        const auto localUrl = backend.prepareClipboardImage();
        const auto path = QUrl(localUrl).toLocalFile();
        if (path.isEmpty() || QImage(path).size() != source.size())
            return EXIT_FAILURE;
        QGuiApplication::clipboard()->clear();
        backend.copyImage(QString(), path);
        const bool copied = QGuiApplication::clipboard()->image().size() == source.size();
        backend.discardClipboardImage(localUrl);
        return copied && !QFileInfo::exists(path) ? EXIT_SUCCESS : EXIT_FAILURE;
    }
    if (smokeTest && screenshotPath.isEmpty()) {
        if (engine.rootObjects().isEmpty())
            return EXIT_FAILURE;
        // A selected rail destination must have a visible circular state in
        // both themes. The previous light colors differed by a single RGB
        // step, so the control technically had a background but looked bare.
        auto *root = engine.rootObjects().constFirst();
        auto *navigationRail = root->findChild<QObject *>(QStringLiteral("navigationRail"));
        auto *selectedNavigation = root->findChild<QObject *>(QStringLiteral("navigationChatsBackground"));
        if (!navigationRail || !selectedNavigation)
            return EXIT_FAILURE;
        const auto railColor = navigationRail->property("color").value<QColor>();
        const auto selectedColor = selectedNavigation->property("color").value<QColor>();
        return qAbs(railColor.lightness() - selectedColor.lightness()) >= 10
            ? EXIT_SUCCESS : EXIT_FAILURE;
    }
    if (!screenshotPath.isEmpty()) {
        QTimer::singleShot(1200, &app, [&engine, screenshotPath] {
            if (engine.rootObjects().isEmpty()) {
                QCoreApplication::exit(EXIT_FAILURE);
                return;
            }
            auto *window = qobject_cast<QQuickWindow *>(engine.rootObjects().constFirst());
            if (window == nullptr || !window->grabWindow().save(screenshotPath)) {
                QCoreApplication::exit(EXIT_FAILURE);
                return;
            }
            QCoreApplication::exit(EXIT_SUCCESS);
        });
    }
    QObject::connect(&instanceServer, &QLocalServer::newConnection, &app, [&] {
        while (auto *socket = instanceServer.nextPendingConnection()) {
            auto activate = [socket, &backend, &engine] {
                const auto chat = QString::fromUtf8(socket->readLine()).trimmed();
                if (!chat.isEmpty())
                    backend.openChat(chat, chat);
                if (!engine.rootObjects().isEmpty()) {
                    if (auto *window = qobject_cast<QWindow *>(engine.rootObjects().constFirst())) {
                        window->show();
                        window->raise();
                        window->requestActivate();
                    }
                }
                socket->disconnectFromServer();
            };
            QObject::connect(socket, &QLocalSocket::readyRead, socket, activate);
            if (socket->bytesAvailable() > 0)
                activate();
        }
    });
    return app.exec();
}
