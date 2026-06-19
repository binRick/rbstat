Name:           rbstat
Version:        %{?_rbstat_version}%{!?_rbstat_version:0.0.0}
Release:        1%{?dist}
Summary:        A Go clone of the classic Linux dstat resource monitor

License:        MIT
URL:            https://github.com/binRick/rbstat
Source0:        rbstat

ExclusiveOS:    linux
# Statically linked Go binary: no runtime dependencies, nothing to debug-split.
AutoReqProv:    no
%global debug_package %{nil}

%description
rbstat is a single static-binary Go clone of the classic Linux dstat
resource-utilization monitor. It samples /proc each interval, diffs the
counters, and prints colorized, dash-padded, fixed-width columns of CPU,
disk, network, paging, system, memory, swap, load average and disk I/O
statistics, matched against dstat's output.

%prep
# Nothing to unpack: Source0 is the prebuilt static binary.

%build
# Nothing to build: the binary is cross-compiled ahead of time.

%install
install -D -m 0755 %{SOURCE0} %{buildroot}%{_bindir}/rbstat

%files
%{_bindir}/rbstat

%changelog
* Thu Jun 18 2026 binRick <4857149+binRick@users.noreply.github.com> - 0.1.0-1
- Initial package: rbstat, a Go clone of classic Linux dstat.
