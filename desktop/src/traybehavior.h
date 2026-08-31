#pragma once

#include <QWindow>

namespace TrayBehavior {

bool shouldHideWindow(QWindow::Visibility visibility, bool trayAvailable);
bool shouldKeepRunning(bool trayAvailable);

} // namespace TrayBehavior
