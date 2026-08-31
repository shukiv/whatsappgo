#include "traybehavior.h"

namespace TrayBehavior {

bool shouldHideWindow(QWindow::Visibility visibility, bool trayAvailable)
{
    return trayAvailable && visibility == QWindow::Minimized;
}

bool shouldKeepRunning(bool trayAvailable)
{
    return trayAvailable;
}

} // namespace TrayBehavior
