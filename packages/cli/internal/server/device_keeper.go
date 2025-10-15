package server

import (
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"path"
	"sync"
	"time"

	"github.com/babelcloud/gbox/packages/cli/internal/cloud"
	"github.com/babelcloud/gbox/packages/cli/internal/server/handlers"
	adb "github.com/basiooo/goadb"
	"github.com/pires/go-proxyproto"
	"github.com/pkg/errors"
	"github.com/vishalkuo/bimap"
	"github.com/xtaci/smux"
)

type DeviceKeeper struct {
	adbClient     *adb.Adb
	deviceWatcher *adb.DeviceWatcher

	adbDeviceBiMap *bimap.BiMap[string, string]
	deviceSessions *sync.Map

	deviceAPI *cloud.DeviceAPI
	apAPI     *cloud.AccessPointAPI

	mu sync.RWMutex
}

func NewDeviceKeeper() (*DeviceKeeper, error) {
	adbClient, err := adb.NewWithConfig(adb.ServerConfig{
		Port: adb.AdbPort,
	})
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create adb client on port %d", adb.AdbPort)
	}
	return &DeviceKeeper{
		adbClient:      adbClient,
		adbDeviceBiMap: bimap.NewBiMap[string, string](),
		deviceSessions: &sync.Map{},
		deviceAPI:      cloud.NewDeviceAPI(),
		apAPI:          cloud.NewAccessPointAPI(),
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
				if err := dm.connectAP(event.Serial); err != nil {
					log.Print(errors.Wrapf(err, "failed to connect device %s to access point", event.Serial))
				}
			case adb.StateOffline:
				if err := dm.disconnectAP(event.Serial); err != nil {
					log.Print(errors.Wrapf(err, "failed to disconnect device %s from access point", event.Serial))
				}
			}
		}
		if dm.deviceWatcher.Err() != nil {
			log.Print(errors.Wrap(dm.deviceWatcher.Err(), "adb device watcher error"))
		}
	}()

	return nil
}

func (dm *DeviceKeeper) Close() {
	if dm.deviceWatcher != nil {
		dm.deviceWatcher.Shutdown()
	}
}

func (dm *DeviceKeeper) getAdbSerialByGboxDeviceId(deviceId string) string {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	serial, ok := dm.adbDeviceBiMap.GetInverse(deviceId)
	if !ok {
		return ""
	}
	return serial
}

func (dm *DeviceKeeper) connectAP(serial string) error {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	serialno, androidId, err := handlers.GetDeviceSerialnoAndAndroidId(serial)
	if err != nil {
		return errors.Wrapf(err, "failed to get device %s serialno and android_id", serial)
	}

	deviceList, err := dm.deviceAPI.GetBySerialnoAndAndroidId(serialno, androidId)
	if err != nil {
		return errors.Wrapf(err, "failed to get GBOX devices with serialno %s and androidId %s", serialno, androidId)
	}
	if len(deviceList.Data) == 0 {
		return errors.Errorf("device %s not registered in GBOX", serial)
	}

	device := deviceList.Data[0]

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
	connectEndpoint.Path = path.Join("/devices", device.Id, "connect")

	token, err := dm.deviceAPI.GenerateAccessPointToken(device.Id, connectEndpoint.String())
	if err != nil {
		return errors.Wrapf(err, "failed to generate access point token")
	}

	session, err := connectAP(connectEndpoint.String(), token.Token, apList.Data[0].Metadata.Protocol, serial)
	if err != nil {
		return errors.Wrapf(err, "failed to connect device %s to GBOX access point", serial)
	}

	dm.adbDeviceBiMap.Insert(serial, device.Id)
	dm.deviceSessions.Store(serial, session)
	go processDeviceSession(session, serial)
	go dm.deviceConnectLiveCheck(serial, session)
	return nil
}

func (dm *DeviceKeeper) disconnectAP(serial string) error {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	rawSession, ok := dm.deviceSessions.Load(serial)
	if ok {
		session := rawSession.(*smux.Session)
		session.Close()
	}

	dm.adbDeviceBiMap.Delete(serial)
	dm.deviceSessions.Delete(serial)
	return nil
}

func (dm *DeviceKeeper) existAdbDevice(serial string) bool {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	_, ok := dm.adbDeviceBiMap.Get(serial)
	return ok
}

func (dm *DeviceKeeper) deviceConnectLiveCheck(serial string, session *smux.Session) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("recovered from device %s DeviceKeeper.deviceConnectLiveCheck goroutine: %v", serial, r)
		}
	}()

	for range time.Tick(3 * time.Second) {
		if !dm.existAdbDevice(serial) {
			return
		}

		if session.IsClosed() {
			go func() {
				if err := dm.connectAP(serial); err != nil {
					dm.disconnectAP(serial)
					log.Print(errors.Wrapf(err, "failed to reconnect device %s to GBOX access point", serial))
				}

			}()
			return
		}
	}
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
	resp, err := http.DefaultClient.Do(req)
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

func processDeviceSession(session *smux.Session, serial string) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("recovered from device %s processDeviceSession goroutine: %v", serial, r)
		}
	}()

	for {
		stream, err := session.AcceptStream()
		if err != nil {
			log.Print(errors.Wrapf(err, "device %s session failed to accept stream", serial))
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
