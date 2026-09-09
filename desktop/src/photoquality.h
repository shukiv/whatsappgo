#pragma once

#include <QString>
#include <QTemporaryFile>
#include <memory>

namespace PhotoQuality {
struct Prepared {
    QString path;
    QString error;
    // The private converted copy lives until the upload callback releases it.
    // The selected original is never overwritten or removed.
    std::shared_ptr<QTemporaryFile> temporary;
};

Prepared prepare(const QString &path, const QString &quality);
Prepared prepareProfilePhoto(const QString &path);
}
