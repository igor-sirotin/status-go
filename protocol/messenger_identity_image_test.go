package protocol

import (
	"context"
	"crypto/ecdsa"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/cenkalti/backoff/v3"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"

	gethbridge "github.com/status-im/status-go/eth-node/bridge/geth"
	"github.com/status-im/status-go/eth-node/crypto"
	"github.com/status-im/status-go/eth-node/types"
	"github.com/status-im/status-go/images"
	"github.com/status-im/status-go/multiaccounts"
	"github.com/status-im/status-go/multiaccounts/settings"
	"github.com/status-im/status-go/params"
	"github.com/status-im/status-go/protocol/protobuf"
	"github.com/status-im/status-go/protocol/requests"
	"github.com/status-im/status-go/protocol/tt"
	"github.com/status-im/status-go/waku"
)

func TestMessengerProfilePictureHandlerSuite(t *testing.T) {
	suite.Run(t, new(MessengerProfilePictureHandlerSuite))
}

type MessengerProfilePictureHandlerSuite struct {
	suite.Suite
	alice *Messenger // client instance of Messenger
	bob   *Messenger // server instance of Messenger

	// If one wants to send messages between different instances of Messenger,
	// a single Waku service should be shared.
	shh    types.Waku
	logger *zap.Logger
}

func (s *MessengerProfilePictureHandlerSuite) SetupSuite() {
	s.logger = tt.MustCreateTestLogger()

	// Setup Waku things
	config := waku.DefaultConfig
	config.MinimumAcceptedPoW = 0
	wakuLogger := s.logger.Named("Waku")
	shh := waku.New(&config, wakuLogger)
	s.shh = gethbridge.NewGethWakuWrapper(shh)
	s.Require().NoError(shh.Start())
}

func (s *MessengerProfilePictureHandlerSuite) TearDownSuite() {
	_ = gethbridge.GetGethWakuFrom(s.shh).Stop()
	_ = s.logger.Sync()
}

func (s *MessengerProfilePictureHandlerSuite) newMessenger(name string) *Messenger {
	m, err := newTestMessenger(s.shh, testMessengerConfig{
		logger: s.logger.Named(fmt.Sprintf("messenger-%s", name)),
		name:   name,
		extraOptions: []Option{
			WithAppSettings(newTestSettings(), params.NodeConfig{}),
		},
	})
	s.Require().NoError(err)

	_, err = m.Start()
	s.Require().NoError(err)

	return m
}

func (s *MessengerProfilePictureHandlerSuite) SetupTest() {
	// Generate Alice Messenger
	s.alice = s.newMessenger("Alice")
	s.bob = s.newMessenger("Bobby")

	// Setup MultiAccount for Alice Messenger
	s.setupMultiAccount(s.alice)
}

func (s *MessengerProfilePictureHandlerSuite) TearDownTest() {
	// Shutdown messengers
	TearDownMessenger(&s.Suite, s.alice)
	s.alice = nil
	TearDownMessenger(&s.Suite, s.bob)
	s.bob = nil
	_ = s.logger.Sync()
}

func (s *MessengerProfilePictureHandlerSuite) setupMultiAccount(m *Messenger) {
	name, err := m.settings.DisplayName()
	s.Require().NoError(err)

	keyUID := m.IdentityPublicKeyString()
	m.account = &multiaccounts.Account{
		Name:   name,
		KeyUID: keyUID,
	}

	err = m.multiAccounts.SaveAccount(*m.account)
	s.NoError(err)
}

func (s *MessengerProfilePictureHandlerSuite) generateAndStoreIdentityImages(m *Messenger) map[string]images.IdentityImage {
	keyUID := m.IdentityPublicKeyString()
	iis := images.SampleIdentityImages()

	err := m.multiAccounts.StoreIdentityImages(keyUID, iis, false)
	s.Require().NoError(err)

	out := make(map[string]images.IdentityImage)

	for _, ii := range iis {
		out[ii.Name] = ii
	}

	s.Require().Contains(out, images.SmallDimName)
	s.Require().Contains(out, images.LargeDimName)

	return out
}

func (s *MessengerProfilePictureHandlerSuite) TestChatIdentity() {
	iis := s.generateAndStoreIdentityImages(s.alice)
	ci, err := s.alice.createChatIdentity(privateChat)
	s.Require().NoError(err)
	s.Require().Exactly(len(iis), len(ci.Images))
}

func (s *MessengerProfilePictureHandlerSuite) TestEncryptDecryptIdentityImagesWithContactPubKeys() {
	smPayload := "hello small image"
	lgPayload := "hello large image"

	ci := protobuf.ChatIdentity{
		Clock: uint64(time.Now().Unix()),
		Images: map[string]*protobuf.IdentityImage{
			"small": {
				Payload: []byte(smPayload),
			},
			"large": {
				Payload: []byte(lgPayload),
			},
		},
	}

	// Make contact keys and Contacts, set the Contacts to added
	contactKeys := make([]*ecdsa.PrivateKey, 10)
	for i := range contactKeys {
		contactKey, err := crypto.GenerateKey()
		s.Require().NoError(err)
		contactKeys[i] = contactKey

		contact, err := BuildContactFromPublicKey(&contactKey.PublicKey)
		s.Require().NoError(err)

		contact.ContactRequestLocalState = ContactRequestStateSent

		s.alice.allContacts.Store(contact.ID, contact)
	}

	// Test EncryptIdentityImagesWithContactPubKeys
	err := EncryptIdentityImagesWithContactPubKeys(ci.Images, s.alice)
	s.Require().NoError(err)

	for _, ii := range ci.Images {
		s.Require().Equal(s.alice.allContacts.Len(), len(ii.EncryptionKeys))
	}
	s.Require().NotEqual([]byte(smPayload), ci.Images["small"].Payload)
	s.Require().NotEqual([]byte(lgPayload), ci.Images["large"].Payload)
	s.Require().True(ci.Images["small"].Encrypted)
	s.Require().True(ci.Images["large"].Encrypted)

	// Test DecryptIdentityImagesWithIdentityPrivateKey
	err = DecryptIdentityImagesWithIdentityPrivateKey(ci.Images, contactKeys[2], &s.alice.identity.PublicKey)
	s.Require().NoError(err)

	s.Require().Equal(smPayload, string(ci.Images["small"].Payload))
	s.Require().Equal(lgPayload, string(ci.Images["large"].Payload))
	s.Require().False(ci.Images["small"].Encrypted)
	s.Require().False(ci.Images["large"].Encrypted)

	// RESET Messenger identity, Contacts and IdentityImage.EncryptionKeys
	s.alice.allContacts = new(contactMap)
	ci.Images["small"].EncryptionKeys = nil
	ci.Images["large"].EncryptionKeys = nil

	// Test EncryptIdentityImagesWithContactPubKeys with no contacts
	err = EncryptIdentityImagesWithContactPubKeys(ci.Images, s.alice)
	s.Require().NoError(err)

	for _, ii := range ci.Images {
		s.Require().Equal(0, len(ii.EncryptionKeys))
	}
	s.Require().NotEqual([]byte(smPayload), ci.Images["small"].Payload)
	s.Require().NotEqual([]byte(lgPayload), ci.Images["large"].Payload)
	s.Require().True(ci.Images["small"].Encrypted)
	s.Require().True(ci.Images["large"].Encrypted)

	// Test DecryptIdentityImagesWithIdentityPrivateKey with no valid identity
	err = DecryptIdentityImagesWithIdentityPrivateKey(ci.Images, contactKeys[2], &s.alice.identity.PublicKey)
	s.Require().NoError(err)

	s.Require().NotEqual([]byte(smPayload), ci.Images["small"].Payload)
	s.Require().NotEqual([]byte(lgPayload), ci.Images["large"].Payload)
	s.Require().True(ci.Images["small"].Encrypted)
	s.Require().True(ci.Images["large"].Encrypted)
}

func (s *MessengerProfilePictureHandlerSuite) TestPictureInPrivateChatOneSided() {
	err := s.bob.settings.SaveSettingField(settings.ProfilePicturesVisibility, settings.ProfilePicturesShowToEveryone)
	s.Require().NoError(err)

	err = s.alice.settings.SaveSettingField(settings.ProfilePicturesVisibility, settings.ProfilePicturesShowToEveryone)
	s.Require().NoError(err)

	bChat := CreateOneToOneChat(s.alice.IdentityPublicKeyString(), s.alice.IdentityPublicKey(), s.alice.transport)
	err = s.bob.SaveChat(bChat)
	s.Require().NoError(err)

	_, err = s.bob.Join(bChat)
	s.Require().NoError(err)

	// Alice sends a message to the public chat
	message := buildTestMessage(*bChat)
	response, err := s.bob.SendChatMessage(context.Background(), message)
	s.Require().NoError(err)
	s.Require().NotNil(response)

	options := func(b *backoff.ExponentialBackOff) {
		b.MaxElapsedTime = 2 * time.Second
	}

	err = tt.RetryWithBackOff(func() error {

		response, err = s.alice.RetrieveAll()
		if err != nil {
			return err
		}
		s.Require().NotNil(response)

		contacts := response.Contacts
		s.logger.Debug("RetryWithBackOff contact data", zap.Any("contacts", contacts))

		if len(contacts) > 0 && len(contacts[0].Images) > 0 {
			s.logger.Debug("", zap.Any("contacts", contacts))
			return nil
		}

		return errors.New("no new contacts with images received")
	}, options)
}

func (s *MessengerProfilePictureHandlerSuite) TestE2eSendingReceivingProfilePicture() {
	profilePicShowSettings := []settings.ProfilePicturesShowToType{
		settings.ProfilePicturesShowToContactsOnly,
		settings.ProfilePicturesShowToEveryone,
		settings.ProfilePicturesShowToNone,
	}

	profilePicViewSettings := []settings.ProfilePicturesVisibilityType{
		settings.ProfilePicturesVisibilityContactsOnly,
		settings.ProfilePicturesVisibilityEveryone,
		settings.ProfilePicturesVisibilityNone,
	}

	isContactFor := map[string][]bool{
		"alice": {true, false},
		"bob":   {true, false},
	}

	chatContexts := []ChatContext{
		publicChat,
		privateChat,
	}

	// TODO see if possible to push each test scenario into a go routine
	for _, cc := range chatContexts {
		for _, ss := range profilePicShowSettings {
			for _, vs := range profilePicViewSettings {
				for _, ac := range isContactFor["alice"] {
					for _, bc := range isContactFor["bob"] {
						args := &e2eArgs{
							chatContext:    cc,
							showToType:     ss,
							visibilityType: vs,
							aliceContact:   ac,
							bobContact:     bc,
						}
						s.Run(args.TestCaseName(s.T()), func() {
							s.testE2eSendingReceivingProfilePicture(args)
						})
					}
				}
			}
		}
	}

	s.SetupTest()
}

func (s *MessengerProfilePictureHandlerSuite) testE2eSendingReceivingProfilePicture(args *e2eArgs) {
	// Generate Alice Messenger
	alice := s.newMessenger("Alice")
	bob := s.newMessenger("Bobby")

	// Setup MultiAccount for Alice Messenger
	s.setupMultiAccount(alice)

	defer func() {
		TearDownMessenger(&s.Suite, alice)
		alice = nil
		TearDownMessenger(&s.Suite, bob)
		bob = nil
		_ = s.logger.Sync()
	}()

	s.logger.Info("testing with criteria:", zap.Any("args", args))
	defer s.logger.Info("Completed testing with criteria:", zap.Any("args", args))

	expectPicture, err := args.resultExpected()
	s.Require().NoError(err)

	s.logger.Debug("expect to receive a profile pic?",
		zap.Bool("result", expectPicture),
		zap.Error(err))

	// Setting up Bob
	err = bob.settings.SaveSettingField(settings.ProfilePicturesVisibility, args.visibilityType)
	s.Require().NoError(err)

	if args.bobContact {
		_, err = bob.AddContact(context.Background(), &requests.AddContact{ID: alice.IdentityPublicKeyString()})
		s.Require().NoError(err)
	}

	// Create Bob's chats
	switch args.chatContext {
	case publicChat:
		// Bob opens up the public chat and joins it
		bChat := CreatePublicChat("status", alice.transport)
		err = bob.SaveChat(bChat)
		s.Require().NoError(err)

		_, err = bob.Join(bChat)
		s.Require().NoError(err)
	case privateChat:
		bChat := CreateOneToOneChat(alice.IdentityPublicKeyString(), alice.IdentityPublicKey(), alice.transport)
		err = bob.SaveChat(bChat)
		s.Require().NoError(err)

		_, err = bob.Join(bChat)
		s.Require().NoError(err)
	default:
		s.Failf("unexpected chat context type", "%s", string(args.chatContext))
	}

	// Setting up Alice
	err = alice.settings.SaveSettingField(settings.ProfilePicturesShowTo, args.showToType)
	s.Require().NoError(err)

	if args.aliceContact {
		_, err = alice.AddContact(context.Background(), &requests.AddContact{ID: bob.IdentityPublicKeyString()})
		s.Require().NoError(err)
	}

	iis := s.generateAndStoreIdentityImages(alice)

	// Create chats
	var aChat *Chat
	switch args.chatContext {
	case publicChat:
		// Alice opens creates a public chat
		aChat = CreatePublicChat("status", alice.transport)
		err = alice.SaveChat(aChat)
		s.Require().NoError(err)

		// Alice sends a message to the public chat
		message := buildTestMessage(*aChat)
		response, err := alice.SendChatMessage(context.Background(), message)
		s.Require().NoError(err)
		s.Require().NotNil(response)
		s.Require().Len(response.messages, 1)

	case privateChat:
		aChat = CreateOneToOneChat(bob.IdentityPublicKeyString(), bob.IdentityPublicKey(), bob.transport)
		err = alice.SaveChat(aChat)
		s.Require().NoError(err)

		_, err = alice.Join(aChat)
		s.Require().NoError(err)

		err = alice.publishContactCode()
		s.Require().NoError(err)

	default:
		s.Failf("unexpected chat context type", "%s", string(args.chatContext))
	}

	// Poll bob to see if he got the chatIdentity
	// Retrieve ChatIdentity
	var contacts []*Contact

	options := func(b *backoff.ExponentialBackOff) {
		b.MaxElapsedTime = 2 * time.Second
	}

	err = tt.RetryWithBackOff(func() error {
		response, err := bob.RetrieveAll()
		if err != nil {
			return err
		}

		contacts = response.Contacts
		if len(contacts) > 0 && len(contacts[0].Images) > 0 {
			return nil
		}

		return errors.New("no new contacts with images received")
	}, options)

	if !expectPicture {
		s.Require().EqualError(err, "no new contacts with images received")
		return
	}

	s.Require().NoError(err)
	s.Require().NotNil(contacts)

	// Check if alice's contact data with profile picture is there
	var contact *Contact
	for _, c := range contacts {
		if c.ID == alice.IdentityPublicKeyString() {
			contact = c
		}
	}
	s.Require().NotNil(contact)

	// Check that Bob now has Alice's profile picture(s)
	switch args.chatContext {
	case publicChat:
		// In public chat context we only need the images.SmallDimName, but also may have the large
		s.Require().GreaterOrEqual(len(contact.Images), 1)
		s.Require().Contains(contact.Images, images.SmallDimName)
		s.Require().Equal(iis[images.SmallDimName].Payload, contact.Images[images.SmallDimName].Payload)

	case privateChat:
		s.Require().Equal(len(contact.Images), 2)
		s.Require().Contains(contact.Images, images.SmallDimName)
		s.Require().Contains(contact.Images, images.LargeDimName)
		s.Require().Equal(iis[images.SmallDimName].Payload, contact.Images[images.SmallDimName].Payload)
		s.Require().Equal(iis[images.LargeDimName].Payload, contact.Images[images.LargeDimName].Payload)
	}
}

type e2eArgs struct {
	chatContext    ChatContext
	showToType     settings.ProfilePicturesShowToType
	visibilityType settings.ProfilePicturesVisibilityType
	aliceContact   bool
	bobContact     bool
}

func (args *e2eArgs) String() string {
	return fmt.Sprintf("ChatContext: %s, ShowTo: %s, Visibility: %s, AliceContact: %t, BobContact: %t",
		string(args.chatContext),
		profilePicShowSettingsMap[args.showToType],
		profilePicViewSettingsMap[args.visibilityType],
		args.aliceContact,
		args.bobContact,
	)
}

func (args *e2eArgs) TestCaseName(t *testing.T) string {
	expected, err := args.resultExpected()
	require.NoError(t, err)

	return fmt.Sprintf("%s-%s-%s-ac.%t-bc.%t-exp.%t",
		string(args.chatContext),
		profilePicShowSettingsMap[args.showToType],
		profilePicViewSettingsMap[args.visibilityType],
		args.aliceContact,
		args.bobContact,
		expected,
	)
}

func (args *e2eArgs) resultExpected() (bool, error) {
	switch args.showToType {
	case settings.ProfilePicturesShowToContactsOnly:
		if args.aliceContact {
			return args.resultExpectedVS()
		}
		return false, nil
	case settings.ProfilePicturesShowToEveryone:
		return args.resultExpectedVS()
	case settings.ProfilePicturesShowToNone:
		return false, nil
	default:
		return false, errors.New("unknown ProfilePicturesShowToType")
	}
}

func (args *e2eArgs) resultExpectedVS() (bool, error) {
	switch args.visibilityType {
	case settings.ProfilePicturesVisibilityContactsOnly:
		return true, nil
	case settings.ProfilePicturesVisibilityEveryone:
		return true, nil
	case settings.ProfilePicturesVisibilityNone:
		// If we are contacts, we save the image regardless
		return args.bobContact, nil
	default:
		return false, errors.New("unknown ProfilePicturesVisibilityType")
	}
}

var profilePicShowSettingsMap = map[settings.ProfilePicturesShowToType]string{
	settings.ProfilePicturesShowToContactsOnly: "ShowToContactsOnly",
	settings.ProfilePicturesShowToEveryone:     "ShowToEveryone",
	settings.ProfilePicturesShowToNone:         "ShowToNone",
}

var profilePicViewSettingsMap = map[settings.ProfilePicturesVisibilityType]string{
	settings.ProfilePicturesVisibilityContactsOnly: "ViewFromContactsOnly",
	settings.ProfilePicturesVisibilityEveryone:     "ViewFromEveryone",
	settings.ProfilePicturesVisibilityNone:         "ViewFromNone",
}
