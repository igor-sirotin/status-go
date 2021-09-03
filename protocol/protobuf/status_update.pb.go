// Code generated by protoc-gen-go. DO NOT EDIT.
// source: status_update.proto

package protobuf

import (
	fmt "fmt"
	proto "github.com/golang/protobuf/proto"
	math "math"
)

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.ProtoPackageIsVersion3 // please upgrade the proto package

type StatusUpdate_StatusType int32

const (
	StatusUpdate_UNKNOWN_STATUS_TYPE StatusUpdate_StatusType = 0
	StatusUpdate_ONLINE              StatusUpdate_StatusType = 1
	StatusUpdate_DO_NOT_DISTURB      StatusUpdate_StatusType = 2
)

var StatusUpdate_StatusType_name = map[int32]string{
	0: "UNKNOWN_STATUS_TYPE",
	1: "ONLINE",
	2: "DO_NOT_DISTURB",
}

var StatusUpdate_StatusType_value = map[string]int32{
	"UNKNOWN_STATUS_TYPE": 0,
	"ONLINE":              1,
	"DO_NOT_DISTURB":      2,
}

func (x StatusUpdate_StatusType) String() string {
	return proto.EnumName(StatusUpdate_StatusType_name, int32(x))
}

func (StatusUpdate_StatusType) EnumDescriptor() ([]byte, []int) {
	return fileDescriptor_911acd91e62cd3d7, []int{0, 0}
}

type StatusUpdate struct {
	Clock                uint64                  `protobuf:"varint,1,opt,name=clock,proto3" json:"clock,omitempty"`
	StatusType           StatusUpdate_StatusType `protobuf:"varint,2,opt,name=status_type,json=statusType,proto3,enum=protobuf.StatusUpdate_StatusType" json:"status_type,omitempty"`
	CustomText           string                  `protobuf:"bytes,3,opt,name=custom_text,json=customText,proto3" json:"custom_text,omitempty"`
	XXX_NoUnkeyedLiteral struct{}                `json:"-"`
	XXX_unrecognized     []byte                  `json:"-"`
	XXX_sizecache        int32                   `json:"-"`
}

func (m *StatusUpdate) Reset()         { *m = StatusUpdate{} }
func (m *StatusUpdate) String() string { return proto.CompactTextString(m) }
func (*StatusUpdate) ProtoMessage()    {}
func (*StatusUpdate) Descriptor() ([]byte, []int) {
	return fileDescriptor_911acd91e62cd3d7, []int{0}
}

func (m *StatusUpdate) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_StatusUpdate.Unmarshal(m, b)
}
func (m *StatusUpdate) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_StatusUpdate.Marshal(b, m, deterministic)
}
func (m *StatusUpdate) XXX_Merge(src proto.Message) {
	xxx_messageInfo_StatusUpdate.Merge(m, src)
}
func (m *StatusUpdate) XXX_Size() int {
	return xxx_messageInfo_StatusUpdate.Size(m)
}
func (m *StatusUpdate) XXX_DiscardUnknown() {
	xxx_messageInfo_StatusUpdate.DiscardUnknown(m)
}

var xxx_messageInfo_StatusUpdate proto.InternalMessageInfo

func (m *StatusUpdate) GetClock() uint64 {
	if m != nil {
		return m.Clock
	}
	return 0
}

func (m *StatusUpdate) GetStatusType() StatusUpdate_StatusType {
	if m != nil {
		return m.StatusType
	}
	return StatusUpdate_UNKNOWN_STATUS_TYPE
}

func (m *StatusUpdate) GetCustomText() string {
	if m != nil {
		return m.CustomText
	}
	return ""
}

func init() {
	proto.RegisterEnum("protobuf.StatusUpdate_StatusType", StatusUpdate_StatusType_name, StatusUpdate_StatusType_value)
	proto.RegisterType((*StatusUpdate)(nil), "protobuf.StatusUpdate")
}

func init() {
	proto.RegisterFile("status_update.proto", fileDescriptor_911acd91e62cd3d7)
}

var fileDescriptor_911acd91e62cd3d7 = []byte{
	// 213 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0xe2, 0x12, 0x2e, 0x2e, 0x49, 0x2c,
	0x29, 0x2d, 0x8e, 0x2f, 0x2d, 0x48, 0x49, 0x2c, 0x49, 0xd5, 0x2b, 0x28, 0xca, 0x2f, 0xc9, 0x17,
	0xe2, 0x00, 0x53, 0x49, 0xa5, 0x69, 0x4a, 0x17, 0x18, 0xb9, 0x78, 0x82, 0xc1, 0x2a, 0x42, 0xc1,
	0x0a, 0x84, 0x44, 0xb8, 0x58, 0x93, 0x73, 0xf2, 0x93, 0xb3, 0x25, 0x18, 0x15, 0x18, 0x35, 0x58,
	0x82, 0x20, 0x1c, 0x21, 0x27, 0x2e, 0x6e, 0xa8, 0x39, 0x25, 0x95, 0x05, 0xa9, 0x12, 0x4c, 0x0a,
	0x8c, 0x1a, 0x7c, 0x46, 0x8a, 0x7a, 0x30, 0x63, 0xf4, 0x90, 0x8d, 0x80, 0x72, 0x42, 0x2a, 0x0b,
	0x52, 0x83, 0xb8, 0x8a, 0xe1, 0x6c, 0x21, 0x79, 0x2e, 0xee, 0xe4, 0xd2, 0xe2, 0x92, 0xfc, 0xdc,
	0xf8, 0x92, 0xd4, 0x8a, 0x12, 0x09, 0x66, 0x05, 0x46, 0x0d, 0xce, 0x20, 0x2e, 0x88, 0x50, 0x48,
	0x6a, 0x45, 0x89, 0x92, 0x2b, 0x17, 0x17, 0x42, 0xab, 0x90, 0x38, 0x97, 0x70, 0xa8, 0x9f, 0xb7,
	0x9f, 0x7f, 0xb8, 0x5f, 0x7c, 0x70, 0x88, 0x63, 0x48, 0x68, 0x70, 0x7c, 0x48, 0x64, 0x80, 0xab,
	0x00, 0x83, 0x10, 0x17, 0x17, 0x9b, 0xbf, 0x9f, 0x8f, 0xa7, 0x9f, 0xab, 0x00, 0xa3, 0x90, 0x10,
	0x17, 0x9f, 0x8b, 0x7f, 0xbc, 0x9f, 0x7f, 0x48, 0xbc, 0x8b, 0x67, 0x70, 0x48, 0x68, 0x90, 0x93,
	0x00, 0x53, 0x12, 0x1b, 0xd8, 0x55, 0xc6, 0x80, 0x00, 0x00, 0x00, 0xff, 0xff, 0xf5, 0x6f, 0xd8,
	0x56, 0xfa, 0x00, 0x00, 0x00,
}