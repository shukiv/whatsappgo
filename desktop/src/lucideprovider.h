#pragma once

#include <QQuickImageProvider>

// Renders the bundled, trusted Lucide subset. QML's image cache shares each
// name/tint/size across controls, including on the software scene graph.
class LucideProvider final : public QQuickImageProvider
{
public:
    LucideProvider();
    QImage requestImage(const QString &id, QSize *size, const QSize &requestedSize) override;
};
