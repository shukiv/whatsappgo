#include "rpcclient.h"

#include "TestSupport.h"
#include <QCoreApplication>
#include <QDir>
#include <QFile>
#include <QLocalServer>
#include <QTemporaryDir>

namespace {
int failures = 0;

void check(bool condition, const char *what)
{
    if (!condition) {
        qWarning("FAILED: %s", what);
        ++failures;
    }
}
}

// The reconnect path deletes the socket before starting a daemon, so that a
// crashed daemon's leftover socket cannot block the new one. Deleting a socket
// a live daemon is still serving orphans that daemon: it keeps running on a
// path nothing can reach, holds its databases open, and the next failed
// connection leaks another one. This is the check that keeps them apart.
int main(int argc, char **argv)
{
    QCoreApplication app(argc, argv);
    QTemporaryDir runtime;
    if (!runtime.isValid())
        return testFatal("could not create a temporary runtime directory");
    qputenv("XDG_RUNTIME_DIR", runtime.path().toUtf8());
    const auto socketDir = QDir(runtime.path()).filePath(QStringLiteral("whatsappgo"));
    if (!QDir().mkpath(socketDir))
        return testFatal("could not create the socket directory", socketDir);

    const auto served = RpcClient::socketPathForProfile(QStringLiteral("live"));
    QLocalServer server;
    if (!server.listen(served))
        return testFatal("could not listen on the socket", served + QStringLiteral(": ") + server.errorString());
    check(RpcClient::backendIsListening(served), "a served socket is reported as live");

    const auto missing = RpcClient::socketPathForProfile(QStringLiteral("absent"));
    check(!RpcClient::backendIsListening(missing), "a socket that was never created is not live");

#ifndef Q_OS_WIN
    // A daemon that died can leave its path behind with nobody accepting on
    // it. That path must not read as live, or no daemon would ever be started.
    const auto stale = RpcClient::socketPathForProfile(QStringLiteral("stale"));
    QFile leftover(stale);
    if (!leftover.open(QIODevice::WriteOnly))
        return testFatal("could not leave a stale socket path behind", stale);
    leftover.close();
    check(!RpcClient::backendIsListening(stale), "a path nobody serves is not live");
#endif

    return failures == 0 ? EXIT_SUCCESS : EXIT_FAILURE;
}
