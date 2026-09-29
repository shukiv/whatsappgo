Name:           whatsappgo
Version:        0.1.12
Release:        1%{?dist}
Summary:        Low-memory native WhatsApp client for Linux
License:        GPL-3.0-or-later
URL:            https://github.com/shukiv/whatsappgo
Source0:        %{name}-%{version}.tar.gz

BuildRequires:  golang >= 1.26
BuildRequires:  cmake
BuildRequires:  ninja-build
BuildRequires:  qt6-qtbase-devel
BuildRequires:  qt6-qtdeclarative-devel
BuildRequires:  qt6-qtmultimedia-devel
BuildRequires:  qt6-qtsvg-devel
# Animated stickers are decoded here rather than by a Qt image plugin.
BuildRequires:  libwebp-devel
Recommends:     notification-daemon
Suggests:       gnome-shell-extension-appindicator

%description
WhatsAppGo combines a native Qt interface with a lightweight Go
backend and the WhatsApp linked-device protocol. The desktop application
starts and owns the bundled backend automatically. It does not embed a browser.

%prep
%autosetup

%build
CGO_ENABLED=0 go build -trimpath -ldflags '-s -w -X main.version=%{version}' -o bin/whatsappd ./cmd/whatsappd
CGO_ENABLED=0 go build -trimpath -ldflags '-s -w -X main.version=%{version}' -o bin/whatsappctl ./cmd/whatsappctl
%cmake -S desktop -G Ninja
%cmake_build

%install
%cmake_install
install -Dm644 packaging/metainfo/org.whatsappgo.Desktop.metainfo.xml %{buildroot}%{_metainfodir}/org.whatsappgo.Desktop.metainfo.xml

%files
%license LICENSE
%{_bindir}/whatsappgo
%{_bindir}/whatsappd
%{_bindir}/whatsappctl
%{_datadir}/applications/org.whatsappgo.Desktop.desktop
%{_datadir}/icons/hicolor/scalable/apps/org.whatsappgo.Desktop.svg
%{_metainfodir}/org.whatsappgo.Desktop.metainfo.xml

%changelog
* Sun Sep 27 2026 WhatsAppGo Contributors <maintainers@whatsappgo.org> - 0.1.12-1
- Remove the app and everything it put on the computer, on request
- Write a message now and send it at a chosen moment
- Read a channel you follow, and list a community joined through its group
- Move through a voice note, and see when one was listened to
- Stop a freshly linked account inventing unread badges and splitting a contact
- Keep message notifications from going missing during a busy spell

* Sat Sep 12 2026 WhatsAppGo Contributors <maintainers@whatsappgo.org> - 0.1.10-1
- Bound photo panning by the photo instead of the viewer around it
- Start a right-to-left contact name beside the avatar, as a Latin one does
- State the Terms of Service and account-ban risk of an unofficial client

* Fri Sep 11 2026 WhatsAppGo Contributors <maintainers@whatsappgo.org> - 0.1.9-1
- Refuse sends to addresses that are not conversations; guard chat export
- Hand refused notifications back to the window and escape markup bodies
- Bound the alert table and fix stale dialogs, drafts, media pages and receipts

* Fri Sep 11 2026 WhatsAppGo Contributors <maintainers@whatsappgo.org> - 0.1.8-1
- Expand groups, mentions, media controls, search and keyboard navigation
- Fix pinned-chat synchronization, sticker recovery and menu placement

* Mon Sep 07 2026 WhatsAppGo Contributors <maintainers@whatsappgo.org> - 0.1.7-1
- Show a stable unread-message divider and count in light and dark themes
- Keep unread boundaries consistent with paged history and read receipts
- Preserve arrivals during chat loading and reject stale opening responses

* Mon Sep 07 2026 WhatsAppGo Contributors <maintainers@whatsappgo.org> - 0.1.6-1
- Restore chat-tail navigation, compact media bubbles and expiring typing indicators
- Preserve attachment replies, isolate image-copy callbacks and correct download accounting
- Open WhatsAppGo GitHub issues directly for bug reporting

* Sun Sep 06 2026 WhatsAppGo Contributors <maintainers@whatsappgo.org> - 0.1.5-1
- Preserve drafts, message metadata and archived media during failures and identity merges
- Fix image wheel zoom, privacy synchronization and scoped media actions
- Route bug reports to the authenticated WhatsAppGo intake

* Sun Aug 30 2026 WhatsAppGo Contributors <maintainers@whatsappgo.org> - 0.1.0-1
- Initial package
