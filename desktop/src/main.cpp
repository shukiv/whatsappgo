#include "rpcclient.h"

#include <QGuiApplication>
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
#include <QFileInfo>
#include <QQuickItem>
#include <QProcess>

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

    QGuiApplication app(argc, argv);
    QGuiApplication::setOrganizationName(QStringLiteral("WhatsAppGo"));
    QGuiApplication::setOrganizationDomain(QStringLiteral("whatsappgo.org"));
    QGuiApplication::setApplicationName(QStringLiteral("WhatsAppGo"));
    QGuiApplication::setDesktopFileName(QStringLiteral("org.whatsappgo.Desktop"));
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
    const auto screenshotPath = parser.value(screenshotOption);
    const bool automatedRun = smokeTest || searchNavigationTest || messageInteractionTest || clipboardImageTest
        || layoutRegressionTest || mediaPreviewTest || chatFilterTest || backendLifecycleTest || resizeRenderingTest
        || messageLayoutTest || !screenshotPath.isEmpty();

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

    RpcClient backend(initialProfile, parser.value(chatOption));
    QQmlApplicationEngine engine;
    engine.rootContext()->setContextProperty(QStringLiteral("backend"), &backend);
    QObject::connect(&engine, &QQmlApplicationEngine::objectCreationFailed, &app,
                     [] { QCoreApplication::exit(EXIT_FAILURE); }, Qt::QueuedConnection);
    engine.loadFromModule(QStringLiteral("org.whatsappgo"), QStringLiteral("Main"));
    if (!engine.rootObjects().isEmpty())
        installResizeRepaintGuard(qobject_cast<QQuickWindow *>(engine.rootObjects().constFirst()));
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
        auto *chatItem = qobject_cast<QQuickItem *>(chatDelegate.get());
        if (!timestamp || !badge || !preview || !chatItem)
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
        return qAbs(timestampRight - badgeRight) <= 1.0
                && qAbs(badge->width() - 24.0) <= 1.0
                && qAbs(badge->height() - 24.0) <= 1.0
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
