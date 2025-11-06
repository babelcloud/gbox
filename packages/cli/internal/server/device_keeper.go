package server

import (
	"crypto/tls"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"path"
	"runtime/debug"
	"sync"
	"time"

	"github.com/babelcloud/gbox/packages/cli/internal/cloud"
	"github.com/babelcloud/gbox/packages/cli/internal/device"
	"github.com/babelcloud/gbox/packages/cli/internal/server/handlers"
	adb "github.com/basiooo/goadb"
	"github.com/dchest/uniuri"
	"github.com/pires/go-proxyproto"
	"github.com/pkg/errors"
	"github.com/vishalkuo/bimap"
	"github.com/xtaci/smux"
	"k8s.io/utils/keymutex"
)

var deviceConnectClient = &http.Client{
	Transport: &http.Transport{
		TLSNextProto: make(map[string]func(authority string, c *tls.Conn) http.RoundTripper),
	},
}

type deviceInfo struct {
	DeviceDTO *handlers.DeviceDTO // Complete device information from handlers package
	ExpiresAt time.Time           // Expiration time
}

type DeviceKeeper struct {
	adbClient     *adb.Adb
	deviceWatcher *adb.DeviceWatcher

	adbDeviceBiMap *bimap.BiMap[string, string]
	deviceSessions *DeviceMap

	deviceAPI *cloud.DeviceAPI
	apAPI     *cloud.AccessPointAPI

	// Device info cache with expiration
	// Key can be serialno, deviceId (TransportID), or regId
	deviceInfoCache map[string]*deviceInfo
	infoCacheMu     sync.RWMutex

	mu         sync.RWMutex
	deviceLock keymutex.KeyMutex
}

func NewDeviceKeeper() (*DeviceKeeper, error) {
	adbClient, err := adb.NewWithConfig(adb.ServerConfig{
		Port: adb.AdbPort,
	})
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create adb client on port %d", adb.AdbPort)
	}
	return &DeviceKeeper{
		adbClient:       adbClient,
		adbDeviceBiMap:  bimap.NewBiMap[string, string](),
		deviceSessions:  NewDeviceMap(),
		deviceAPI:       cloud.NewDeviceAPI(),
		apAPI:           cloud.NewAccessPointAPI(),
		deviceInfoCache: make(map[string]*deviceInfo),
		deviceLock:      keymutex.NewHashed(10000),
	}, nil
}

func (dm *DeviceKeeper) Start() error {
	if err := dm.adbClient.StartServer(); err != nil {
		return errors.Wrapf(err, "failed to start ")
	}

	dm.deviceWatcher = dm.adbClient.NewDeviceWatcher()
	go func() {
		for event := range dm.deviceWatcher.C() {
			log.Printf("device event: %s %s -> %s", event.Serial, event.OldState, event.NewState)
			switch event.NewState {
			case adb.StateOnline:
				go func() {
					defer func() {
						if r := recover(); r != nil {
							log.Printf("recovered from connectAP: device %s event %s goroutine: %v", event.Serial, event.NewState, r)
						}
					}()
					if err := dm.connectAP(event.Serial); err != nil {
						log.Print(errors.Wrapf(err, "failed to connect device %s to access point", event.Serial))
					}
				}()

			case adb.StateOffline:
				go func() {
					defer func() {
						if r := recover(); r != nil {
							log.Printf("recovered from disconnectAPForce: device %s event %s goroutine: %v: %s", event.Serial, event.NewState, r, string(debug.Stack()))
						}
					}()

					if err := dm.disconnectAPForce(event.Serial); err != nil {
						log.Print(errors.Wrapf(err, "failed to disconnect device %s from access point", event.Serial))
					}
				}()
			}
		}
		if dm.deviceWatcher.Err() != nil {
			log.Print(errors.Wrap(dm.deviceWatcher.Err(), "adb device watcher error"))
		}
	}()

	// Reconnect all registered devices (both Android and desktop)
	go func() {
		// Give adb watcher some time to detect online devices first
		time.Sleep(2 * time.Second)
		if err := dm.ReconnectRegisteredDevices(); err != nil {
			log.Printf("Failed to reconnect registered devices: %v", err)
		}
	}()

	return nil
}

func (dm *DeviceKeeper) Close() {
	if dm.deviceWatcher != nil {
		dm.deviceWatcher.Shutdown()
	}
}

// getSerialByDeviceId gets the device serial (serialno) by device ID (gbox device ID)
// Supports both Android and desktop devices by checking both adbDeviceBiMap and deviceInfoCache
func (dm *DeviceKeeper) getSerialByDeviceId(deviceId string) string {
	// First, try to get from adbDeviceBiMap (for Android devices that are connected)
	dm.mu.RLock()
	serial, ok := dm.adbDeviceBiMap.GetInverse(deviceId)
	dm.mu.RUnlock()
	if ok && serial != "" {
		return serial
	}

	// If not found in adbDeviceBiMap, try to get from device info cache
	// This works for both Android and desktop devices
	dm.infoCacheMu.RLock()
	defer dm.infoCacheMu.RUnlock()

	// Search through cache to find device with matching ID
	for _, info := range dm.deviceInfoCache {
		if time.Now().After(info.ExpiresAt) {
			continue // Skip expired entries
		}
		if info.DeviceDTO != nil && info.DeviceDTO.ID == deviceId {
			// Found matching device ID, return its serialno
			if info.DeviceDTO.Serialno != "" {
				return info.DeviceDTO.Serialno
			}
		}
	}

	// Also try to match by TransportID or RegId as fallback
	for _, info := range dm.deviceInfoCache {
		if time.Now().After(info.ExpiresAt) {
			continue
		}
		if info.DeviceDTO != nil {
			if info.DeviceDTO.TransportID == deviceId || info.DeviceDTO.RegId == deviceId {
				if info.DeviceDTO.Serialno != "" {
					return info.DeviceDTO.Serialno
				}
			}
		}
	}

	return ""
}

// getAdbSerialByGboxDeviceId is kept for backward compatibility
// Use getSerialByDeviceId instead for universal device support
func (dm *DeviceKeeper) getAdbSerialByGboxDeviceId(deviceId string) string {
	return dm.getSerialByDeviceId(deviceId)
}

func (dm *DeviceKeeper) connectAP(serial string) error {
	devMgr := device.NewManager("android")
	ids, err := devMgr.GetIdentifiers(serial)
	var deviceList *cloud.DeviceList

	if err != nil {
		// Fall back: treat input as a deviceId for non-ADB devices (desktop)
		// Try to find device by deviceId directly
		deviceList, err = dm.deviceAPI.GetByRegId(serial)
		if err != nil || len(deviceList.Data) == 0 {
			// If not found by regId, treat serial as deviceId directly
			return dm.connectAPUsingDeviceId(serial, serial, "", "")
		}
		// Found device by regId, use it
		dev := deviceList.Data[0]
		deviceType := dev.Metadata.DeviceType
		osType := dev.Metadata.OsType
		return dm.connectAPUsingDeviceId(serial, dev.Id, deviceType, osType)
	}

	// Android device: get serialno and androidId
	serialno := ids.SerialNo
	var androidId string
	if ids.AndroidID != nil {
		androidId = *ids.AndroidID
	}

	deviceList, err = dm.deviceAPI.GetBySerialnoAndAndroidId(serialno, androidId)
	if err != nil {
		return errors.Wrapf(err, "failed to get GBOX devices with serialno %s and androidId %s", serialno, androidId)
	}
	if len(deviceList.Data) == 0 {
		return errors.Errorf("device %s not registered in GBOX", serial)
	}

	dev := deviceList.Data[0]
	deviceType := dev.Metadata.DeviceType
	osType := dev.Metadata.OsType
	return dm.connectAPUsingDeviceId(serial, dev.Id, deviceType, osType)
}

// connectAPUsingDeviceId establishes AP connection using known gbox deviceId.
// key is used as the map/session key and logging serial (adb serial or deviceId).
// deviceType and osType are stored for device type-specific handling.
func (dm *DeviceKeeper) connectAPUsingDeviceId(key string, deviceId string, deviceType string, osType string) error {
	dm.deviceLock.LockKey(key)
	defer dm.deviceLock.UnlockKey(key)

	apList, err := dm.apAPI.List()
	if err != nil {
		return errors.Wrapf(err, "failed to list access point")
	}
	if len(apList.Data) == 0 {
		return errors.Errorf("no access point found")
	}

	connectEndpoint, err := url.Parse(apList.Data[0].Endpoint)
	if err != nil {
		return errors.Wrapf(err, "invalid access point endpoint %s", apList.Data[0].Endpoint)
	}
	connectEndpoint.Path = path.Join("/devices", deviceId, "connect")

	token, err := dm.deviceAPI.GenerateAccessPointToken(deviceId, connectEndpoint.String())
	if err != nil {
		return errors.Wrapf(err, "failed to generate access point token")
	}

	mux, err := connectAP(connectEndpoint.String(), token.Token, apList.Data[0].Metadata.Protocol, key)
	if err != nil {
		return errors.Wrapf(err, "failed to connect device %s to GBOX access point", key)
	}

	session := dm.addDevice(key, &DeviceSession{
		Mux:        mux,
		Serial:     key,
		DeviceType: deviceType,
		OsType:     osType,
	}, deviceId)

	// Create minimal device info for cache (full info will be updated when device list is queried)
	// This ensures we can look up device platform immediately after connection
	dto := &handlers.DeviceDTO{
		ID:         deviceId,
		Serialno:   key,
		Platform:   deviceType, // deviceType is actually "mobile" or "desktop" here
		OS:         osType,
		DeviceType: "", // Will be filled when device list is queried
	}
	dm.updateDeviceInfo(dto)

	go dm.processDeviceSession(session, key)
	return nil
}

func (dm *DeviceKeeper) disconnectAP(session *DeviceSession) error {
	dm.deviceLock.LockKey(session.Serial)
	defer dm.deviceLock.UnlockKey(session.Serial)

	session.Mux.Close()

	dm.delDevice(session)
	return nil
}

func (dm *DeviceKeeper) disconnectAPForce(serial string) error {
	session, ok := dm.getDevice(serial)
	if ok {
		dm.deviceLock.LockKey(serial)
		defer dm.deviceLock.UnlockKey(serial)

		if session != nil {
			if session.Mux != nil {
				session.Mux.Close()
			}
			dm.delDevice(session)
		}
	}
	return nil
}

func (dm *DeviceKeeper) getDevice(serial string) (*DeviceSession, bool) {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	session, ok := dm.deviceSessions.Get(serial)
	return session, ok
}

// IsDeviceConnected checks if a device is currently connected to AP
func (dm *DeviceKeeper) IsDeviceConnected(serial string) bool {
	_, ok := dm.getDevice(serial)
	return ok
}

func (dm *DeviceKeeper) hasDevice(serial string) bool {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	_, ok := dm.adbDeviceBiMap.Get(serial)
	return ok
}

func (dm *DeviceKeeper) addDevice(serial string, deviceSession *DeviceSession, gboxDeviceId string) *DeviceSession {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	session := dm.deviceSessions.Set(serial, deviceSession)
	dm.adbDeviceBiMap.Insert(serial, gboxDeviceId)
	return session
}

func (dm *DeviceKeeper) delDevice(session *DeviceSession) {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	if dm.deviceSessions.Delete(session.Serial, session.Token) {
		dm.adbDeviceBiMap.Delete(session.Serial)
		// Note: We don't delete from platform cache here to allow it to expire naturally
		// This helps with reconnection scenarios
	}
}

// updateDeviceInfo updates the device info cache with complete device information
// Cache expires after 1 hour of inactivity
// Stores device info under multiple keys: serialno, TransportID, ID, and regId (if available)
func (dm *DeviceKeeper) updateDeviceInfo(dto *handlers.DeviceDTO) {
	dm.infoCacheMu.Lock()
	defer dm.infoCacheMu.Unlock()

	info := &deviceInfo{
		DeviceDTO: dto,
		ExpiresAt: time.Now().Add(1 * time.Hour),
	}

	// Store under multiple keys for flexible lookup
	if dto.Serialno != "" {
		dm.deviceInfoCache[dto.Serialno] = info
	}
	if dto.TransportID != "" && dto.TransportID != dto.Serialno {
		dm.deviceInfoCache[dto.TransportID] = info
	}
	if dto.ID != "" {
		dm.deviceInfoCache[dto.ID] = info
	}
	if dto.RegId != "" {
		dm.deviceInfoCache[dto.RegId] = info
	}
}

// GetDeviceInfo gets the complete device information from cache by serialno, deviceId, or regId
// Returns nil if not found or expired
func (dm *DeviceKeeper) GetDeviceInfo(key string) *handlers.DeviceDTO {
	dm.infoCacheMu.RLock()
	defer dm.infoCacheMu.RUnlock()

	info, ok := dm.deviceInfoCache[key]
	if !ok {
		return nil
	}

	// Check if expired
	if time.Now().After(info.ExpiresAt) {
		// Clean up expired entry (async)
		go dm.cleanupExpiredDeviceInfo(key)
		return nil
	}

	return info.DeviceDTO
}

// cleanupExpiredDeviceInfo removes expired entries from cache
func (dm *DeviceKeeper) cleanupExpiredDeviceInfo(key string) {
	dm.infoCacheMu.Lock()
	defer dm.infoCacheMu.Unlock()

	info, ok := dm.deviceInfoCache[key]
	if ok && time.Now().After(info.ExpiresAt) {
		delete(dm.deviceInfoCache, key)
	}
}

// CleanupExpiredDeviceInfos removes all expired entries from cache
func (dm *DeviceKeeper) CleanupExpiredDeviceInfos() {
	dm.infoCacheMu.Lock()
	defer dm.infoCacheMu.Unlock()

	now := time.Now()
	for key, info := range dm.deviceInfoCache {
		if now.After(info.ExpiresAt) {
			delete(dm.deviceInfoCache, key)
		}
	}
}

// ReconnectRegisteredDevices reconnects all registered devices on server start
// This is called after server startup to restore device connections
func (dm *DeviceKeeper) ReconnectRegisteredDevices() error {
	// Get all registered devices from cloud
	deviceList, err := dm.deviceAPI.GetAll()
	if err != nil {
		return errors.Wrap(err, "failed to get registered devices from cloud")
	}

	log.Printf("Attempting to reconnect %d registered device(s)", len(deviceList.Data))

	for _, dev := range deviceList.Data {
		// Skip if device is already connected
		if dm.IsDeviceConnected(dev.Metadata.Serialno) {
			log.Printf("Device %s (%s) already connected, skipping", dev.Id, dev.Metadata.Serialno)
			continue
		}

		deviceType := dev.Metadata.DeviceType
		osType := dev.Metadata.OsType
		serialno := dev.Metadata.Serialno

		// For Android devices, wait for adb watcher to handle connection
		// Only reconnect desktop devices here
		if deviceType == "desktop" && serialno != "" {
			log.Printf("Reconnecting desktop device %s (%s)", dev.Id, serialno)
			go func(id, sn, dt, ot string) {
				if err := dm.connectAPUsingDeviceId(sn, id, dt, ot); err != nil {
					log.Printf("Failed to reconnect device %s: %v", id, err)
				} else {
					log.Printf("Successfully reconnected device %s", id)
				}
			}(dev.Id, serialno, deviceType, osType)
		}
	}

	return nil
}

func connectAP(url, token, protocol, serial string) (*smux.Session, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create request to connect to access point %s via %s protocol", url, protocol)
	}
	req.Header.Set("Connection", "upgrade")
	req.Header.Set("Upgrade", protocol)
	req.Header.Set("User-Agent", "GBOX-cli")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := deviceConnectClient.Do(req)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to connect to access point %s via %s protocol", url, protocol)
	}
	if resp.StatusCode != http.StatusSwitchingProtocols {
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		return nil, errors.Wrapf(err, "access point does not switch protocol, respond %d: %v", resp.StatusCode, string(body))
	}

	log.Printf("device %s connected access point %s via %s protocol", serial, url, protocol)
	rwCloser, ok := resp.Body.(io.ReadWriteCloser)
	if !ok {
		defer resp.Body.Close()
		return nil, errors.Errorf("failed to convert access point connection into read write closer")
	}

	session, err := smux.Server(rwCloser, nil)
	if err != nil {
		defer resp.Body.Close()
		return nil, errors.Wrapf(err, "failed to create smux session from access point connection")
	}

	return session, nil
}

func (dm *DeviceKeeper) processDeviceSession(session *DeviceSession, serial string) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("recovered from device %s processDeviceSession goroutine: %v: %s", serial, r, string(debug.Stack()))
		}
	}()

	for {
		stream, err := session.Mux.AcceptStream()
		if err != nil && dm.hasDevice(serial) {
			log.Print(errors.Wrapf(err, "device %s session closed. try to reconnect", serial))
			go func() {
				// Use stored mapping to get gbox deviceId for reconnection
				deviceId, _ := dm.adbDeviceBiMap.Get(serial)
				// Preserve device type information from the session
				if err := dm.connectAPUsingDeviceId(serial, deviceId, session.DeviceType, session.OsType); err != nil {
					dm.disconnectAP(session)
					log.Print(errors.Wrapf(err, "failed to reconnect device %s to GBOX access point", serial))
				}
			}()
			return
		}
		log.Printf("device %s stream %d accepted", serial, stream.ID())
		go processSessionStream(stream, serial)
	}
}

func processSessionStream(stream *smux.Stream, serial string) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("recovered from device %s stream %d processStream goroutine: %v", serial, stream.ID(), r)
		}
	}()

	local := proxyproto.NewConn(stream)
	defer log.Printf("device %s stream %d closed", serial, stream.ID())
	defer local.Close()

	proxyHeader := local.ProxyHeader()
	log.Print(proxyHeader.DestinationAddr.String())

	host, port, err := net.SplitHostPort(proxyHeader.DestinationAddr.String())
	if err != nil {
		log.Print(err)
		return
	}
	if host == "0.0.0.0" {
		tlvs, err := proxyHeader.TLVs()
		if err != nil {
			log.Print(err)
			return
		}
		for _, tlv := range tlvs {
			if tlv.Type == proxyproto.PP2_TYPE_AUTHORITY {
				host = string(tlv.Value)
				break
			}
		}
		ips, err := net.LookupIP(host)
		if err != nil {
			log.Print(err)
			return
		}
		host = ips[0].String()
	}

	remote, err := net.Dial("tcp", net.JoinHostPort(host, port))
	if err != nil {
		log.Print(err)
		return
	}
	defer remote.Close()

	var wg sync.WaitGroup
	defer wg.Wait()

	wg.Go(func() {
		defer local.Close()
		if _, err := io.Copy(local, remote); err != nil {
			log.Printf("device %s stream %d local <- remote: %v", serial, stream.ID(), err)
		}
	})
	wg.Go(func() {
		defer remote.Close()
		if _, err := io.Copy(remote, local); err != nil {
			log.Printf("device %s stream %d remote <- local: %v", serial, stream.ID(), err)
		}
	})
}

type DeviceSession struct {
	Mux        *smux.Session
	Token      string
	Serial     string
	DeviceType string // mobile or desktop
	OsType     string // android, linux, windows, macos
}

type DeviceMap struct {
	sessions map[string]*DeviceSession
	mu       sync.RWMutex
}

func NewDeviceMap() *DeviceMap {
	return &DeviceMap{
		sessions: map[string]*DeviceSession{},
	}
}

func (dm *DeviceMap) Len() int {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	return len(dm.sessions)
}

func (dm *DeviceMap) Get(serial string) (*DeviceSession, bool) {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	session, ok := dm.sessions[serial]
	return session, ok
}

func (dm *DeviceMap) Set(serial string, session *DeviceSession) *DeviceSession {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	session.Token = uniuri.NewLen(32)
	dm.sessions[serial] = session
	return session
}

func (dm *DeviceMap) Delete(serial, token string) bool {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	if session, ok := dm.sessions[serial]; ok && session.Token == token {
		delete(dm.sessions, serial)
		return true
	}
	return false
}

func (dm *DeviceMap) DeleteForce(serial string) {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	delete(dm.sessions, serial)
}
