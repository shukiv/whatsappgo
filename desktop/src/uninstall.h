#pragma once

#include <QList>
#include <QString>
#include <QStringList>

// Removing this program is the reader's business, not a support question. The
// app knows where it put everything - accounts, message history, cached
// attachments, settings, sockets, and the desktop entry an AppImage leaves
// behind - so it is the thing that can take them away again.
struct UninstallTarget {
    QString path;
    QString description;
    qint64 bytes = 0;
    bool holdsAccounts = false;
};

struct UninstallPlan {
    QList<UninstallTarget> targets;
    QStringList accounts;
    QString appImage;
    bool somethingIsRunning = false;
};

// planUninstall finds what is actually on this computer. It reads; it removes
// nothing.
UninstallPlan planUninstall();

// runUninstall prints the plan, asks for the word "yes" unless assumeYes, and
// removes what it listed. keepAccounts spares the accounts and their history -
// everything else still goes. Returns the process exit code.
int runUninstall(const UninstallPlan &plan, bool assumeYes, bool keepAccounts);
