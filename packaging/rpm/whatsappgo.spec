Name:           whatsappgo
Version:        0.1.0
Release:        1%{?dist}
Summary:        Low-memory native WhatsApp client for Linux
License:        MPL-2.0
URL:            https://github.com/shuki/whatsappgo
Source0:        %{name}-%{version}.tar.gz

BuildRequires:  golang >= 1.26
BuildRequires:  cmake
BuildRequires:  ninja-build
BuildRequires:  qt6-qtbase-devel
BuildRequires:  qt6-qtdeclarative-devel
BuildRequires:  qt6-qtmultimedia-devel
BuildRequires:  kf6-kirigami-devel

%description
WhatsAppGo combines a native Qt/Kirigami interface with a lightweight Go
backend and the WhatsApp linked-device protocol. The desktop application
starts and owns the bundled backend automatically. It does not embed a browser.

%prep
%autosetup

%build
CGO_ENABLED=0 go build -trimpath -ldflags '-s -w -X main.version=%{version}' -o bin/whatsappd ./cmd/whatsappd
%cmake -S desktop -G Ninja
%cmake_build

%install
%cmake_install
install -Dm644 packaging/metainfo/org.whatsappgo.Desktop.metainfo.xml %{buildroot}%{_metainfodir}/org.whatsappgo.Desktop.metainfo.xml

%files
%license LICENSE
%{_bindir}/whatsappgo
%{_bindir}/whatsappd
%{_datadir}/applications/org.whatsappgo.Desktop.desktop
%{_datadir}/icons/hicolor/scalable/apps/org.whatsappgo.Desktop.svg
%{_metainfodir}/org.whatsappgo.Desktop.metainfo.xml

%changelog
* Sun Aug 30 2026 WhatsAppGo Contributors <maintainers@whatsappgo.org> - 0.1.0-1
- Initial package
