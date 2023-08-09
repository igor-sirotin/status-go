package protocol

import (
	"testing"

	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"

	gethcommon "github.com/ethereum/go-ethereum/common"
	hexutil "github.com/ethereum/go-ethereum/common/hexutil"

	gethbridge "github.com/status-im/status-go/eth-node/bridge/geth"
	"github.com/status-im/status-go/eth-node/types"
	"github.com/status-im/status-go/protocol/common"
	"github.com/status-im/status-go/protocol/protobuf"
	"github.com/status-im/status-go/protocol/tt"
	"github.com/status-im/status-go/waku"
)

func TestOwnerWithoutCommunityKeyCommunityEventsSuite(t *testing.T) {
	suite.Run(t, new(OwnerWithoutCommunityKeyCommunityEventsSuite))
}

type OwnerWithoutCommunityKeyCommunityEventsSuite struct {
	suite.Suite
	controlNode              *Messenger
	ownerWithoutCommunityKey *Messenger
	alice                    *Messenger
	// If one wants to send messages between different instances of Messenger,
	// a single Waku service should be shared.
	shh            types.Waku
	logger         *zap.Logger
	mockedBalances map[uint64]map[gethcommon.Address]map[gethcommon.Address]*hexutil.Big // chainID, account, token, balance
}

func (s *OwnerWithoutCommunityKeyCommunityEventsSuite) GetControlNode() *Messenger {
	return s.controlNode
}

func (s *OwnerWithoutCommunityKeyCommunityEventsSuite) GetEventSender() *Messenger {
	return s.ownerWithoutCommunityKey
}

func (s *OwnerWithoutCommunityKeyCommunityEventsSuite) GetMember() *Messenger {
	return s.alice
}

func (s *OwnerWithoutCommunityKeyCommunityEventsSuite) GetSuite() *suite.Suite {
	return &s.Suite
}

func (s *OwnerWithoutCommunityKeyCommunityEventsSuite) SetupTest() {
	s.logger = tt.MustCreateTestLogger()

	config := waku.DefaultConfig
	config.MinimumAcceptedPoW = 0
	shh := waku.New(&config, s.logger)
	s.shh = gethbridge.NewGethWakuWrapper(shh)
	s.Require().NoError(shh.Start())

	s.controlNode = s.newMessenger("", []string{})
	s.ownerWithoutCommunityKey = s.newMessenger("qwerty", []string{commmunitiesEventsEventSenderAddress})
	s.alice = s.newMessenger("", []string{})
	_, err := s.controlNode.Start()
	s.Require().NoError(err)
	_, err = s.ownerWithoutCommunityKey.Start()
	s.Require().NoError(err)
	_, err = s.alice.Start()
	s.Require().NoError(err)

	s.mockedBalances = createMockedWalletBalance(&s.Suite)
}

func (s *OwnerWithoutCommunityKeyCommunityEventsSuite) TearDownTest() {
	s.Require().NoError(s.controlNode.Shutdown())
	s.Require().NoError(s.ownerWithoutCommunityKey.Shutdown())
	s.Require().NoError(s.alice.Shutdown())
	_ = s.logger.Sync()
}

func (s *OwnerWithoutCommunityKeyCommunityEventsSuite) newMessenger(password string, walletAddresses []string) *Messenger {
	return newMessenger(&s.Suite, s.shh, s.logger, password, walletAddresses, &s.mockedBalances)
}

func (s *OwnerWithoutCommunityKeyCommunityEventsSuite) TestOwnerEditCommunityDescription() {
	community := setUpCommunityAndRoles(s, protobuf.CommunityMember_ROLE_OWNER)
	editCommunityDescription(s, community)
}

func (s *OwnerWithoutCommunityKeyCommunityEventsSuite) TestOwnerCreateEditDeleteChannels() {
	community := setUpCommunityAndRoles(s, protobuf.CommunityMember_ROLE_OWNER)

	testCreateEditDeleteChannels(s, community)
}

func (s *OwnerWithoutCommunityKeyCommunityEventsSuite) TestOwnerCreateEditDeleteBecomeMemberPermission() {
	community := setUpCommunityAndRoles(s, protobuf.CommunityMember_ROLE_OWNER)
	testCreateEditDeleteBecomeMemberPermission(s, community, protobuf.CommunityTokenPermission_BECOME_MEMBER)
}

func (s *OwnerWithoutCommunityKeyCommunityEventsSuite) TestOwnerCreateEditDeleteBecomeAdminPermission() {
	community := setUpCommunityAndRoles(s, protobuf.CommunityMember_ROLE_OWNER)
	testCreateEditDeleteBecomeMemberPermission(s, community, protobuf.CommunityTokenPermission_BECOME_ADMIN)
}

func (s *OwnerWithoutCommunityKeyCommunityEventsSuite) TestOwnerCreateEditDeleteBecomeTokenMasterPermission() {
	community := setUpCommunityAndRoles(s, protobuf.CommunityMember_ROLE_OWNER)
	testCreateEditDeleteBecomeMemberPermission(s, community, protobuf.CommunityTokenPermission_BECOME_TOKEN_MASTER)
}

func (s *OwnerWithoutCommunityKeyCommunityEventsSuite) TestOwnerAcceptMemberRequestToJoinResponseSharedWithOtherEventSenders() {
	additionalOwner := s.newMessenger("", []string{})
	community := setUpOnRequestCommunityAndRoles(s, protobuf.CommunityMember_ROLE_OWNER, []*Messenger{additionalOwner})
	// set up additional user that will send request to join
	user := s.newMessenger("", []string{})
	testAcceptMemberRequestToJoinResponseSharedWithOtherEventSenders(s, community, user, additionalOwner)
}

func (s *OwnerWithoutCommunityKeyCommunityEventsSuite) TestOwnerAcceptMemberRequestToJoinNotConfirmedByControlNode() {
	community := setUpOnRequestCommunityAndRoles(s, protobuf.CommunityMember_ROLE_OWNER, []*Messenger{})
	// set up additional user that will send request to join
	user := s.newMessenger("", []string{})
	testAcceptMemberRequestToJoinNotConfirmedByControlNode(s, community, user)
}

func (s *OwnerWithoutCommunityKeyCommunityEventsSuite) TestOwnerAcceptMemberRequestToJoin() {
	community := setUpOnRequestCommunityAndRoles(s, protobuf.CommunityMember_ROLE_OWNER, []*Messenger{})

	// set up additional user that will send request to join
	user := s.newMessenger("", []string{})
	testAcceptMemberRequestToJoin(s, community, user)
}

func (s *OwnerWithoutCommunityKeyCommunityEventsSuite) TestOwnerRejectMemberRequestToJoinResponseSharedWithOtherEventSenders() {
	additionalOwner := s.newMessenger("", []string{})
	community := setUpOnRequestCommunityAndRoles(s, protobuf.CommunityMember_ROLE_OWNER, []*Messenger{additionalOwner})
	// set up additional user that will send request to join
	user := s.newMessenger("", []string{})
	testAcceptMemberRequestToJoinResponseSharedWithOtherEventSenders(s, community, user, additionalOwner)
}

func (s *OwnerWithoutCommunityKeyCommunityEventsSuite) TestOwnerRejectMemberRequestToJoinNotConfirmedByControlNode() {
	community := setUpOnRequestCommunityAndRoles(s, protobuf.CommunityMember_ROLE_OWNER, []*Messenger{})
	// set up additional user that will send request to join
	user := s.newMessenger("", []string{})
	testRejectMemberRequestToJoinNotConfirmedByControlNode(s, community, user)
}

func (s *OwnerWithoutCommunityKeyCommunityEventsSuite) TestOwnerRejectMemberRequestToJoin() {
	community := setUpOnRequestCommunityAndRoles(s, protobuf.CommunityMember_ROLE_OWNER, []*Messenger{})

	// set up additional user that will send request to join
	user := s.newMessenger("", []string{})
	testRejectMemberRequestToJoin(s, community, user)
}

func (s *OwnerWithoutCommunityKeyCommunityEventsSuite) TestOwnerRequestToJoinStateCannotBeOverridden() {
	additionalOwner := s.newMessenger("", []string{})
	community := setUpOnRequestCommunityAndRoles(s, protobuf.CommunityMember_ROLE_OWNER, []*Messenger{additionalOwner})

	// set up additional user that will send request to join
	user := s.newMessenger("", []string{})
	testEventSenderCannotOverrideRequestToJoinState(s, community, user, additionalOwner)
}

func (s *OwnerWithoutCommunityKeyCommunityEventsSuite) TestOwnerControlNodeHandlesMultipleEventSenderRequestToJoinDecisions() {
	additionalOwner := s.newMessenger("", []string{})
	community := setUpOnRequestCommunityAndRoles(s, protobuf.CommunityMember_ROLE_OWNER, []*Messenger{additionalOwner})

	// set up additional user that will send request to join
	user := s.newMessenger("", []string{})
	testControlNodeHandlesMultipleEventSenderRequestToJoinDecisions(s, community, user, additionalOwner)
}

func (s *OwnerWithoutCommunityKeyCommunityEventsSuite) TestOwnerCreateEditDeleteCategories() {
	community := setUpCommunityAndRoles(s, protobuf.CommunityMember_ROLE_OWNER)
	testCreateEditDeleteCategories(s, community)
}

func (s *OwnerWithoutCommunityKeyCommunityEventsSuite) TestOwnerReorderChannelsAndCategories() {
	community := setUpCommunityAndRoles(s, protobuf.CommunityMember_ROLE_OWNER)
	testReorderChannelsAndCategories(s, community)
}

func (s *OwnerWithoutCommunityKeyCommunityEventsSuite) TestOwnerKickOwnerWithoutCommunityKey() {
	community := setUpCommunityAndRoles(s, protobuf.CommunityMember_ROLE_OWNER)
	testEventSenderKickTheSameRole(s, community)
}

func (s *OwnerWithoutCommunityKeyCommunityEventsSuite) TestOwnerKickControlNode() {
	community := setUpCommunityAndRoles(s, protobuf.CommunityMember_ROLE_OWNER)
	testEventSenderKickControlNode(s, community)
}

func (s *OwnerWithoutCommunityKeyCommunityEventsSuite) TestOwnerKickMember() {
	community := setUpCommunityAndRoles(s, protobuf.CommunityMember_ROLE_OWNER)
	kickMember(s, community.ID(), common.PubkeyToHex(&s.alice.identity.PublicKey))
}

func (s *OwnerWithoutCommunityKeyCommunityEventsSuite) TestOwnerBanOwnerWithoutCommunityKey() {
	community := setUpCommunityAndRoles(s, protobuf.CommunityMember_ROLE_OWNER)
	testOwnerBanTheSameRole(s, community)
}

func (s *OwnerWithoutCommunityKeyCommunityEventsSuite) TestOwnerBanControlNode() {
	community := setUpCommunityAndRoles(s, protobuf.CommunityMember_ROLE_OWNER)
	testOwnerBanControlNode(s, community)
}

func (s *OwnerWithoutCommunityKeyCommunityEventsSuite) TestOwnerBanUnbanMember() {
	community := setUpCommunityAndRoles(s, protobuf.CommunityMember_ROLE_OWNER)
	testBanUnbanMember(s, community)
}

func (s *OwnerWithoutCommunityKeyCommunityEventsSuite) TestOwnerDeleteAnyMessageInTheCommunity() {
	community := setUpCommunityAndRoles(s, protobuf.CommunityMember_ROLE_OWNER)
	testDeleteAnyMessageInTheCommunity(s, community)
}

func (s *OwnerWithoutCommunityKeyCommunityEventsSuite) TestOwnerPinMessage() {
	community := setUpCommunityAndRoles(s, protobuf.CommunityMember_ROLE_OWNER)
	testEventSenderPinMessage(s, community)
}

func (s *OwnerWithoutCommunityKeyCommunityEventsSuite) TestOwnerAddCommunityToken() {
	community := setUpCommunityAndRoles(s, protobuf.CommunityMember_ROLE_OWNER)
	testEventSenderAddedCommunityToken(s, community)
}

func (s *OwnerWithoutCommunityKeyCommunityEventsSuite) TestMemberReceiveOwnerEventsWhenControlNodeOffline() {
	community := setUpCommunityAndRoles(s, protobuf.CommunityMember_ROLE_OWNER)
	testMemberReceiveEventsWhenControlNodeOffline(s, community)
}