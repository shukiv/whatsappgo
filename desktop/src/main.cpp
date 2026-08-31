#include "rpcclient.h"

#include <QApplication>
#include <QAction>
#include <QClipboard>
#include <QCommandLineParser>
#include <QDebug>
#include <QDir>
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

#include <cstdlib>

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
    QCommandLineOption desktopIntegrationTestOption(QStringLiteral("desktop-integration-test"), QStringLiteral("Verify system tray and notification integration"));
    parser.addOption(desktopIntegrationTestOption);
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
    const bool desktopIntegrationTest = parser.isSet(desktopIntegrationTestOption);
    const auto screenshotPath = parser.value(screenshotOption);
    const bool automatedRun = smokeTest || searchNavigationTest || messageInteractionTest || clipboardImageTest
        || layoutRegressionTest || mediaPreviewTest || chatFilterTest || backendLifecycleTest || resizeRenderingTest
        || messageLayoutTest || desktopIntegrationTest || !screenshotPath.isEmpty();

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

    const bool trayEnabled = !automatedRun && QSystemTrayIcon::isSystemTrayAvailable();
    if (!automatedRun)
        app.setQuitOnLastWindowClosed(!trayEnabled);

    RpcClient backend(initialProfile, parser.value(chatOption), trayEnabled);
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

    QString lastNotificationChat;
    QString lastNotificationTitle;
    // Qt's offscreen platform has no tray host and some versions crash while
    // tearing down widget-backed tray menus. Construct this integration only
    // in an interactive session where the platform reports a real tray.
    std::unique_ptr<QMenu> trayMenu;
    std::unique_ptr<QSystemTrayIcon> trayIcon;
    if (trayEnabled) {
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
        if (applicationWindow != nullptr) {
            QObject::connect(applicationWindow, &QWindow::visibilityChanged, &app,
                             [updateToggleAction](QWindow::Visibility) { updateToggleAction(); });
        }
        QObject::connect(trayIcon.get(), &QSystemTrayIcon::activated, &app,
                         [activateWindow](QSystemTrayIcon::ActivationReason reason) {
                             if (reason == QSystemTrayIcon::Trigger || reason == QSystemTrayIcon::DoubleClick)
                                 activateWindow();
                         });
        QObject::connect(&backend, &RpcClient::notificationRequested, &app,
                         [icon = trayIcon.get(), &applicationIcon, &lastNotificationChat, &lastNotificationTitle]
                         (const QString &chatJid, const QString &title, const QString &body) {
                             lastNotificationChat = chatJid;
                             lastNotificationTitle = title;
                             icon->showMessage(title, body, applicationIcon, 8000);
                         });
        QObject::connect(trayIcon.get(), &QSystemTrayIcon::messageClicked, &app,
                         [&backend, activateWindow, &lastNotificationChat, &lastNotificationTitle] {
                             if (!lastNotificationChat.isEmpty())
                                 backend.openChat(lastNotificationChat, lastNotificationTitle);
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
        updateToggleAction();
        updateTrayStatus();
        trayIcon->show();
    }

    if (desktopIntegrationTest) {
        return !applicationIcon.isNull() && !trayEnabled && trayIcon == nullptr && trayMenu == nullptr
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
        QQmlComponent component(&engine, QUrl(QStringLiteral("qrc:/qt/qml/org/whatsappgo/qml/MessageDelegate.qml")));
        const QVariantMap message{
            {QStringLiteral("id"), QStringLiteral("message-test")},
            {QStringLiteral("body"), QStringLiteral("Copy this https://example.com/path?q=1")},
            {QStringLiteral("kind"), QStringLiteral("text")},
            {QStringLiteral("from_me"), false},
            {QStringLiteral("timestamp"), 0},
            {QStringLiteral("status"), QStringLiteral("received")},
        };
        std::unique_ptr<QObject> delegate(component.createWithInitialProperties({
            {QStringLiteral("width"), 800},
            {QStringLiteral("modelData"), message},
        }));
        if (!delegate)
            return EXIT_FAILURE;
        auto *body = delegate->findChild<QObject *>(QStringLiteral("messageBody"));
        if (!body)
            return EXIT_FAILURE;
        const auto rendered = body->property("text").toString();
        const bool selected = QMetaObject::invokeMethod(body, "selectAll")
            && !body->property("selectedText").toString().isEmpty();
        return body->property("selectByMouse").toBool()
                && selected
                && rendered.contains(QStringLiteral("href=\"https://example.com/path?q=1\""))
            ? EXIT_SUCCESS
            : EXIT_FAILURE;
    }
    if (layoutRegressionTest) {
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
        return qAbs(timestampRight - badgeRight) <= 2.0
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
            require(shapeBubble->width() <= 420.0,
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
    if (mediaPreviewTest) {
        if (engine.rootObjects().isEmpty())
            return EXIT_FAILURE;
        QImage source(80, 120, QImage::Format_ARGB32_Premultiplied);
        source.fill(QColor(QStringLiteral("#25D366")));
        QGuiApplication::clipboard()->setImage(source);
        auto *root = engine.rootObjects().constFirst();
        const bool invoked = QMetaObject::invokeMethod(root, "prepareClipboardPaste");
        QCoreApplication::processEvents();
        auto *preview = root->findChild<QObject *>(QStringLiteral("mediaPreviewOverlay"));
        if (!invoked || !preview || !preview->property("previewActive").toBool())
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
    if (smokeTest && screenshotPath.isEmpty())
        return engine.rootObjects().isEmpty() ? EXIT_FAILURE : EXIT_SUCCESS;
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
