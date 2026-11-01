//go:build windows

package gameversion

import (
	"encoding/binary"
	"errors"
	"strings"
	"syscall"
	"unicode/utf16"

	"golang.org/x/sys/windows/registry"
)

// The official Windows distribution is an Inno Setup package. Every silent
// /DIR=... run stores its target as Inno's "previous install directory" under
// HKEY_CURRENT_USER and leaves an Uninstall entry behind. Both outlive the
// launcher: when the user later runs the installer manually, its path page
// prefills the launcher's data folder instead of the default location (issue
// #239). The state is snapshotted before the silent run and restored after it:
// keys that existed before (a real Vintage Story installation) get their
// values back, keys the silent run created are removed.
const (
	innoValuePrefix        = "Inno Setup: "
	innoUninstallSuffix    = "_is1"
	innoUninstallRoot      = `SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall`
	innoDisplayNamePattern = "vintage story"
)

type registryValue struct {
	dataType uint32
	text     string   // SZ, EXPAND_SZ
	number   uint64   // DWORD, QWORD
	binary   []byte   // BINARY
	list     []string // MULTI_SZ
}

type registryKeySnapshot struct {
	path    string
	existed bool
	values  map[string]registryValue
}

type installerState struct {
	targetPath string
	appID      string
	directory  *registryKeySnapshot
	uninstall  *registryKeySnapshot
}

// snapshotInnoState records the Inno Setup registry state the silent installer
// run is about to overwrite, so Restore can put it back afterwards.
func snapshotInnoState(targetPath string) *installerState {
	state := &installerState{targetPath: targetPath}
	state.appID = findInnoAppID(targetPath)
	if state.appID == "" {
		return state
	}
	state.directory = snapshotRegistryKey(`SOFTWARE\`+state.appID, keepInnoValues)
	state.uninstall = snapshotRegistryKey(innoUninstallRoot+`\`+state.appID+innoUninstallSuffix, nil)
	return state
}

// Restore reverts the registry changes a silent installer run left behind.
// When the keys did not exist before the run they are removed entirely,
// otherwise their previous values are written back.
func (state *installerState) Restore() error {
	appID := state.appID
	if appID == "" {
		// The installer created its state during this run; resolve the
		// application ID from the install location it just recorded.
		appID = findInnoAppID(state.targetPath)
		if appID == "" {
			return nil
		}
	}
	var restoreErrs []error
	directory := state.directory
	if directory == nil {
		directory = &registryKeySnapshot{path: `SOFTWARE\` + appID}
	}
	uninstall := state.uninstall
	if uninstall == nil {
		uninstall = &registryKeySnapshot{path: innoUninstallRoot + `\` + appID + innoUninstallSuffix}
	}
	if err := directory.restore(keepInnoValues); err != nil {
		restoreErrs = append(restoreErrs, err)
	}
	if err := uninstall.restore(nil); err != nil {
		restoreErrs = append(restoreErrs, err)
	}
	return errors.Join(restoreErrs...)
}

// findInnoAppID resolves the Inno Setup application ID of the official
// installer, matching either the install location this launcher installs to or
// the Vintage Story display name of a real installation.
func findInnoAppID(targetPath string) string {
	root, err := registry.OpenKey(registry.CURRENT_USER, innoUninstallRoot, registry.ENUMERATE_SUB_KEYS)
	if err != nil {
		return ""
	}
	defer root.Close()
	names, err := root.ReadSubKeyNames(-1)
	if err != nil {
		return ""
	}
	for _, name := range names {
		base, found := strings.CutSuffix(name, innoUninstallSuffix)
		if !found {
			continue
		}
		key, err := registry.OpenKey(registry.CURRENT_USER, innoUninstallRoot+`\`+name, registry.QUERY_VALUE)
		if err != nil {
			continue
		}
		location, _, _ := key.GetStringValue("InstallLocation")
		displayName, _, _ := key.GetStringValue("DisplayName")
		key.Close()
		if strings.EqualFold(strings.TrimSpace(location), targetPath) ||
			strings.Contains(strings.ToLower(displayName), innoDisplayNamePattern) {
			return base
		}
	}
	return ""
}

func keepInnoValues(name string) bool {
	return strings.HasPrefix(name, innoValuePrefix)
}

func snapshotRegistryKey(path string, filter func(string) bool) *registryKeySnapshot {
	snapshot := &registryKeySnapshot{path: path}
	key, err := registry.OpenKey(registry.CURRENT_USER, path, registry.QUERY_VALUE)
	if err != nil {
		return snapshot
	}
	defer key.Close()
	names, err := key.ReadValueNames(-1)
	if err != nil {
		return snapshot
	}
	snapshot.existed = true
	snapshot.values = make(map[string]registryValue)
	for _, name := range names {
		if filter != nil && !filter(name) {
			continue
		}
		if value, ok := readRegistryValue(key, name); ok {
			snapshot.values[name] = value
		}
	}
	return snapshot
}

// restore writes the snapshotted values back and removes entries the
// installer added. Values not matched by keep are left untouched; when keep is
// nil every extra value is deleted. When the key did not exist before the
// installer ran it is removed entirely.
func (snapshot *registryKeySnapshot) restore(keep func(string) bool) error {
	if !snapshot.existed {
		return deleteRegistryTree(registry.CURRENT_USER, snapshot.path)
	}
	key, err := registry.OpenKey(
		registry.CURRENT_USER, snapshot.path, registry.QUERY_VALUE|registry.SET_VALUE)
	if err != nil {
		if errors.Is(err, registry.ErrNotExist) {
			return nil
		}
		return err
	}
	defer key.Close()
	var restoreErrs []error
	for name, value := range snapshot.values {
		if err := value.writeTo(key, name); err != nil {
			restoreErrs = append(restoreErrs, err)
		}
	}
	names, err := key.ReadValueNames(-1)
	if err != nil {
		restoreErrs = append(restoreErrs, err)
		return errors.Join(restoreErrs...)
	}
	for _, name := range names {
		if _, restored := snapshot.values[name]; restored {
			continue
		}
		if keep != nil && !keep(name) {
			continue
		}
		if err := key.DeleteValue(name); err != nil && !errors.Is(err, registry.ErrNotExist) {
			restoreErrs = append(restoreErrs, err)
		}
	}
	return errors.Join(restoreErrs...)
}

func (value registryValue) writeTo(key registry.Key, name string) error {
	switch value.dataType {
	case registry.SZ:
		return key.SetStringValue(name, value.text)
	case registry.EXPAND_SZ:
		return key.SetExpandStringValue(name, value.text)
	case registry.DWORD:
		return key.SetDWordValue(name, uint32(value.number))
	case registry.QWORD:
		return key.SetQWordValue(name, value.number)
	case registry.BINARY:
		return key.SetBinaryValue(name, value.binary)
	case registry.MULTI_SZ:
		return key.SetStringsValue(name, value.list)
	default:
		return nil
	}
}

func readRegistryValue(key registry.Key, name string) (registryValue, bool) {
	required, dataType, err := key.GetValue(name, nil)
	if err != nil && !errors.Is(err, registry.ErrShortBuffer) {
		return registryValue{}, false
	}
	data := make([]byte, max(required, 0))
	if _, _, err := key.GetValue(name, data); err != nil {
		return registryValue{}, false
	}
	return decodeRegistryValue(dataType, data), true
}

func decodeRegistryValue(dataType uint32, data []byte) registryValue {
	value := registryValue{dataType: dataType}
	switch dataType {
	case registry.SZ, registry.EXPAND_SZ:
		value.text = decodeString(data)
	case registry.DWORD:
		if len(data) >= 4 {
			value.number = uint64(binary.LittleEndian.Uint32(data))
		}
	case registry.QWORD:
		if len(data) >= 8 {
			value.number = binary.LittleEndian.Uint64(data)
		}
	case registry.BINARY:
		value.binary = append([]byte(nil), data...)
	case registry.MULTI_SZ:
		value.list = decodeMultiString(data)
	}
	return value
}

// decodeString decodes a NUL-terminated UTF-16 registry string.
func decodeString(data []byte) string {
	if len(data) < 2 {
		return ""
	}
	units := make([]uint16, len(data)/2)
	for index := range units {
		units[index] = binary.LittleEndian.Uint16(data[index*2:])
	}
	return syscall.UTF16ToString(units)
}

func decodeMultiString(data []byte) []string {
	if len(data) < 2 {
		return nil
	}
	units := make([]uint16, len(data)/2)
	for index := range units {
		units[index] = binary.LittleEndian.Uint16(data[index*2:])
	}
	var result []string
	start := 0
	for index, unit := range units {
		if unit != 0 {
			continue
		}
		if index > start {
			result = append(result, string(utf16.Decode(units[start:index])))
		}
		start = index + 1
	}
	return result
}

// deleteRegistryTree removes a registry key with all of its subkeys and
// values. Missing keys are treated as already deleted.
func deleteRegistryTree(root registry.Key, path string) error {
	key, err := registry.OpenKey(root, path, registry.ENUMERATE_SUB_KEYS)
	if err != nil {
		if errors.Is(err, registry.ErrNotExist) {
			return nil
		}
		return err
	}
	subkeys, err := key.ReadSubKeyNames(-1)
	key.Close()
	if err != nil {
		return err
	}
	for _, subkey := range subkeys {
		if err := deleteRegistryTree(root, path+`\`+subkey); err != nil {
			return err
		}
	}
	return registry.DeleteKey(root, path)
}
