package server

import (
	"crypto/ed25519"
	"crypto/rand"
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
	"github.com/dchest/uniuri"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	"github.com/vishalkuo/bimap"
	"golang.org/x/crypto/ssh"
)

type DeviceKeeper struct {
	adbClient     *adb.Adb
	deviceWatcher *adb.DeviceWatcher

	adbDeviceBiMap *bimap.BiMap[string, string]
	deviceSessions *DeviceMap

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
		deviceSessions: NewDeviceMap(),
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
				if err := dm.disconnectAPForce(event.Serial); err != nil {
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

	sshConn, sshChan, err := connectAP(connectEndpoint.String(), token.Token, apList.Data[0].Metadata.Protocol, serial)
	if err != nil {
		return errors.Wrapf(err, "failed to connect device %s to GBOX access point", serial)
	}

	session := dm.deviceSessions.Set(serial, &DeviceSession{
		Conn: sshConn, 
		NewChannel: sshChan,
		Serial: serial,
	})

	dm.adbDeviceBiMap.Insert(serial, device.Id)
	go processDeviceSession(session, serial)
	go dm.deviceConnectLiveCheck(serial, session)
	return nil
}

func (dm *DeviceKeeper) disconnectAP(session *DeviceSession) error {
	session.Conn.Close()

	dm.mu.Lock()
	defer dm.mu.Unlock()

	if dm.deviceSessions.Delete(session.Serial, session.Token) {
		dm.adbDeviceBiMap.Delete(session.Serial)
	}
	return nil
}

func (dm *DeviceKeeper) disconnectAPForce(serial string) error {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	session, ok := dm.deviceSessions.Get(serial)
	if ok {
		session.Conn.Close()
	}

	dm.adbDeviceBiMap.Delete(serial)
	dm.deviceSessions.DeleteForce(serial)
	return nil
}

func (dm *DeviceKeeper) existAdbDevice(serial string) bool {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	_, ok := dm.adbDeviceBiMap.Get(serial)
	return ok
}

func (dm *DeviceKeeper) deviceConnectLiveCheck(serial string, session *DeviceSession) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("recovered from device %s DeviceKeeper.deviceConnectLiveCheck goroutine: %v", serial, r)
		}
	}()

	if err := session.Conn.Wait(); err != nil {
		log.Printf("device %s session is dead: %v", serial, err)

		if !dm.existAdbDevice(serial) {
			return
		}

		go func() {
			if err := dm.connectAP(serial); err != nil {
				dm.disconnectAP(session)
				log.Print(errors.Wrapf(err, "failed to reconnect device %s to GBOX access point", serial))
			}

		}()
	}
}

func connectAP(url, token, protocol, serial string) (*ssh.ServerConn, <-chan ssh.NewChannel, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "failed to create request to connect to access point %s via %s protocol", url, protocol)
	}
	req.Header.Set("Connection", "upgrade")
	req.Header.Set("Upgrade", protocol)
	req.Header.Set("User-Agent", "GBOX-cli")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "failed to connect to access point %s via %s protocol", url, protocol)
	}
	if resp.StatusCode != http.StatusSwitchingProtocols {
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		return nil, nil, errors.Wrapf(err, "access point does not switch protocol, respond %d: %v", resp.StatusCode, string(body))
	}

	log.Printf("device %s connected access point %s via %s protocol", serial, url, protocol)
	rwCloser, ok := resp.Body.(io.ReadWriteCloser)
	if !ok {
		defer resp.Body.Close()
		return nil, nil, errors.Errorf("failed to convert access point connection into read write closer")
	}

	sshConfig := &ssh.ServerConfig{
		NoClientAuth: true,
	}
	sshConfig.AddHostKey(sshSigner)
	sshConn, sshChan, sshReq, err := ssh.NewServerConn(&proxyServerConn{
		ReadWriteCloser: rwCloser,
		laddr: &net.TCPAddr{
			IP:   net.IPv4zero,
			Port: 0,
		},
		raddr: &net.TCPAddr{
			IP:   net.IPv4zero,
			Port: 0,
		},
	}, sshConfig)
	if err != nil {
		defer resp.Body.Close()
		return nil, nil, errors.Wrapf(err, "failed to create ssh connection from access point connection")
	}

	go ssh.DiscardRequests(sshReq)

	return sshConn, sshChan, nil
}

func processDeviceSession(session *DeviceSession, serial string) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("recovered from device %s processDeviceSession goroutine: %v", serial, r)
		}
	}()

	for newChannel := range session.NewChannel {

		if newChannel.ChannelType() == "gbox-device-proxy" {
			target := &ProxyTarget{}
			if err := ssh.Unmarshal(newChannel.ExtraData(), target); err != nil {
				log.Print(errors.Wrapf(err, "failed to umarshal channel data, device %s", serial))
				newChannel.Reject(ssh.ConnectionFailed, "unable to unmarshal data")
				continue
			}

			channel, reqChan, err := newChannel.Accept()
			if err != nil {
				log.Print(errors.Wrapf(err, "failed to accept channel on device %s", serial))
				continue
			}
			go ssh.DiscardRequests(reqChan)
			
			channelId := uuid.New()
			log.Printf("device %s channel %s accepted", serial, channelId.String())
			go processChannel(channel, channelId.String(), serial, target)
		} else {
			log.Printf("device %s receive unknown channel type %s", serial, newChannel.ChannelType())
			newChannel.Reject(ssh.UnknownChannelType, "unknonw channel type")
		}
	}
}

func processChannel(channel ssh.Channel, channelId, serial string, target *ProxyTarget) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("recovered from device %s channel %s processStream goroutine: %v", serial, channelId, r)
		}
	}()

	defer log.Printf("device %s channel %s closed", serial, channelId)
	defer channel.Close()

	log.Printf("device %s channel %s proxy target: %#v", serial, channelId, target)

	host, port, err := net.SplitHostPort(target.Host)
	if err != nil {
		log.Print(errors.Wrapf(err, "device %s channel %d invalid target host %s", serial, channelId, target.Host))
		return
	}
	ip := net.ParseIP(host)
	if ip == nil {
		ips, err := net.LookupIP(host)
		if err != nil {
			log.Print(errors.Wrapf(err, "failed to look up host %s", host))
			return
		}
		ip = ips[0]
	}

	remote, err := net.Dial("tcp", net.JoinHostPort(ip.String(), port))
	if err != nil {
		log.Print(err)
		return
	}
	defer remote.Close()

	var wg sync.WaitGroup
	defer wg.Wait()

	wg.Go(func() {
		defer channel.Close()
		if _, err := io.Copy(channel, remote); err != nil {
			log.Printf("device %s channel %s local <- remote: %v", serial, channelId, err)
		}
	})
	wg.Go(func() {
		defer remote.Close()
		if _, err := io.Copy(remote, channel); err != nil {
			log.Printf("device %s channel %s remote <- local: %v", serial, channelId, err)
		}
	})
}

type DeviceSession struct {
	Conn *ssh.ServerConn
	NewChannel <-chan ssh.NewChannel
	Token  string
	Serial string
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

var sshSigner ssh.Signer

func init() {
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		log.Fatal(errors.Wrapf(err, "failed to generate ed25519 key for ssh server"))
	}
	signer, err := ssh.NewSignerFromKey(privateKey)
	if err != nil {
		log.Fatal(errors.Wrapf(err, "failed to create signer for ssh server"))
	}
	sshSigner = signer
}

type proxyServerConn struct {
	io.ReadWriteCloser
	laddr, raddr net.Addr
}

func (t *proxyServerConn) LocalAddr() net.Addr {
	return t.laddr
}

func (t *proxyServerConn) RemoteAddr() net.Addr {
	return t.raddr
}

func (t *proxyServerConn) SetDeadline(deadline time.Time) error {
	if err := t.SetReadDeadline(deadline); err != nil {
		return err
	}
	return t.SetWriteDeadline(deadline)
}

func (t *proxyServerConn) SetReadDeadline(deadline time.Time) error {
	return errors.New("ssh: tcpChan: deadline not supported")
}

func (t *proxyServerConn) SetWriteDeadline(deadline time.Time) error {
	return errors.New("ssh: tcpChan: deadline not supported")
}

type ProxyTarget struct {
	Host string
}
