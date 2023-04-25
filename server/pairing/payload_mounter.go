package pairing

import (
	"go.uber.org/zap"

	"github.com/status-im/status-go/api"
	"github.com/status-im/status-go/multiaccounts"
)

type PayloadMounter interface {
	PayloadLocker

	// Mount Loads the payload into the PayloadManager's state
	Mount() error

	// ToSend returns an outbound safe (encrypted) payload
	ToSend() []byte
}

type PayloadLoader interface {
	Load() error
}

type BasePayloadMounter struct {
	*PayloadLockPayload
	*PayloadToSend

	loader     PayloadLoader
	marshaller ProtobufMarshaller
	encryptor  *PayloadEncryptor
}

func NewBasePayloadMounter(loader PayloadLoader, marshaller ProtobufMarshaller, e *PayloadEncryptor) *BasePayloadMounter {
	return &BasePayloadMounter{
		PayloadLockPayload: &PayloadLockPayload{e},
		PayloadToSend:      &PayloadToSend{e},
		loader:             loader,
		marshaller:         marshaller,
		encryptor:          e,
	}
}

// Mount loads and prepares the payload to be stored in the PayloadLoader's state ready for later access
func (bpm *BasePayloadMounter) Mount() error {
	err := bpm.loader.Load()
	if err != nil {
		return err
	}

	p, err := bpm.marshaller.MarshalProtobuf()
	if err != nil {
		return err
	}

	return bpm.encryptor.encrypt(p)
}

/*
|--------------------------------------------------------------------------
| AccountPayload
|--------------------------------------------------------------------------
|
| AccountPayloadMounter, AccountPayloadLoader and AccountPayloadMarshaller
|
*/

// NewAccountPayloadMounter generates a new and initialised AccountPayload flavoured BasePayloadMounter
// responsible for the whole lifecycle of an AccountPayload
func NewAccountPayloadMounter(pe *PayloadEncryptor, config *SenderConfig, logger *zap.Logger) (*BasePayloadMounter, error) {
	l := logger.Named("AccountPayloadLoader")
	l.Debug("fired", zap.Any("config", config))

	pe = pe.Renew()

	// A new SHARED AccountPayload
	p := new(AccountPayload)
	apl, err := NewAccountPayloadLoader(p, config)
	if err != nil {
		return nil, err
	}

	return NewBasePayloadMounter(
		apl,
		NewPairingPayloadMarshaller(p, l),
		pe,
	), nil
}

// AccountPayloadLoader is responsible for loading, parsing and validating AccountPayload data
type AccountPayloadLoader struct {
	*AccountPayload

	multiaccountsDB *multiaccounts.Database
	keystorePath    string
	keyUID          string
}

func NewAccountPayloadLoader(p *AccountPayload, config *SenderConfig) (*AccountPayloadLoader, error) {
	ppr := &AccountPayloadLoader{
		AccountPayload: p,
	}

	if config == nil {
		return ppr, nil
	}

	ppr.multiaccountsDB = config.DB
	ppr.keyUID = config.KeyUID
	ppr.password = config.Password
	ppr.keystorePath = config.KeystorePath
	return ppr, nil
}

func (apl *AccountPayloadLoader) Load() error {
	apl.keys = make(map[string][]byte)
	err := loadKeys(apl.keys, apl.keystorePath)
	if err != nil {
		return err
	}

	err = validateKeys(apl.keys, apl.password)
	if err != nil {
		return err
	}

	apl.multiaccount, err = apl.multiaccountsDB.GetAccount(apl.keyUID)
	if err != nil {
		return err
	}

	return nil
}

/*
|--------------------------------------------------------------------------
| RawMessagePayload
|--------------------------------------------------------------------------
|
| RawMessagePayloadMounter and RawMessageLoader
|
*/

// NewRawMessagePayloadMounter generates a new and initialised RawMessagePayload flavoured BasePayloadMounter
// responsible for the whole lifecycle of an RawMessagePayload
func NewRawMessagePayloadMounter(logger *zap.Logger, pe *PayloadEncryptor, backend *api.GethStatusBackend, config *SenderConfig) *BasePayloadMounter {
	pe = pe.Renew()
	payload := NewRawMessagesPayload()

	return NewBasePayloadMounter(
		NewRawMessageLoader(backend, payload, config),
		NewRawMessagePayloadMarshaller(payload),
		pe,
	)
}

type RawMessageLoader struct {
	payload               *RawMessagesPayload
	syncRawMessageHandler *SyncRawMessageHandler
	keyUID                string
	deviceType            string
}

func NewRawMessageLoader(backend *api.GethStatusBackend, payload *RawMessagesPayload, config *SenderConfig) *RawMessageLoader {
	return &RawMessageLoader{
		syncRawMessageHandler: NewSyncRawMessageHandler(backend),
		payload:               payload,
		keyUID:                config.KeyUID,
		deviceType:            config.DeviceType,
	}
}

func (r *RawMessageLoader) Load() (err error) {
	r.payload.rawMessages, r.payload.subAccounts, r.payload.setting, err = r.syncRawMessageHandler.PrepareRawMessage(r.keyUID, r.deviceType)
	return err
}

/*
|--------------------------------------------------------------------------
| InstallationPayload
|--------------------------------------------------------------------------
|
| InstallationPayloadMounter and InstallationPayloadLoader
|
*/

// NewInstallationPayloadMounter generates a new and initialised InstallationPayload flavoured BasePayloadMounter
// responsible for the whole lifecycle of an InstallationPayload
func NewInstallationPayloadMounter(pe *PayloadEncryptor, backend *api.GethStatusBackend, deviceType string) *BasePayloadMounter {
	pe = pe.Renew()
	payload := NewRawMessagesPayload()

	return NewBasePayloadMounter(
		NewInstallationPayloadLoader(backend, payload, deviceType),
		NewRawMessagePayloadMarshaller(payload),
		pe,
	)
}

type InstallationPayloadLoader struct {
	payload               *RawMessagesPayload
	syncRawMessageHandler *SyncRawMessageHandler
	deviceType            string
}

func NewInstallationPayloadLoader(backend *api.GethStatusBackend, payload *RawMessagesPayload, deviceType string) *InstallationPayloadLoader {
	return &InstallationPayloadLoader{
		payload:               payload,
		syncRawMessageHandler: NewSyncRawMessageHandler(backend),
		deviceType:            deviceType,
	}
}

func (r *InstallationPayloadLoader) Load() error {
	rawMessageCollector := new(RawMessageCollector)
	err := r.syncRawMessageHandler.CollectInstallationData(rawMessageCollector, r.deviceType)
	if err != nil {
		return err
	}
	rms := rawMessageCollector.convertToSyncRawMessage()
	r.payload.rawMessages = rms.RawMessages
	return nil
}

/*
|--------------------------------------------------------------------------
| PayloadMounters
|--------------------------------------------------------------------------
|
| Funcs for all PayloadMounters AccountPayloadMounter, RawMessagePayloadMounter and InstallationPayloadMounter
|
*/

// NewPayloadMounters returns PayloadMounter s configured to handle local pairing transfers of:
//   - AccountPayload, RawMessagePayload and InstallationPayload
func NewPayloadMounters(logger *zap.Logger, pe *PayloadEncryptor, backend *api.GethStatusBackend, config *SenderConfig) (PayloadMounter, PayloadMounter, PayloadMounterReceiver, error) {
	am, err := NewAccountPayloadMounter(pe, config, logger)
	if err != nil {
		return nil, nil, nil, err
	}
	rmm := NewRawMessagePayloadMounter(logger, pe, backend, config)
	imr := NewInstallationPayloadMounterReceiver(pe, backend, config.DeviceType)
	return am, rmm, imr, nil
}