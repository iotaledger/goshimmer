// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.27.1
// 	protoc        v3.12.4
// source: packages/network/warpsync/proto/message.proto

package proto

import (
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	reflect "reflect"
	sync "sync"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type Packet struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// Types that are assignable to Body:
	//
	//	*Packet_EpochBlocksStart
	//	*Packet_EpochBlocksBatch
	//	*Packet_EpochBlocksEnd
	//	*Packet_EpochBlocksRequest
	//	*Packet_EpochCommitment
	//	*Packet_EpochCommitmentRequest
	//	*Packet_Negotiation
	Body isPacket_Body `protobuf_oneof:"body"`
}

func (x *Packet) Reset() {
	*x = Packet{}
	if protoimpl.UnsafeEnabled {
		mi := &file_packages_network_warpsync_proto_message_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Packet) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Packet) ProtoMessage() {}

func (x *Packet) ProtoReflect() protoreflect.Message {
	mi := &file_packages_network_warpsync_proto_message_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Packet.ProtoReflect.Descriptor instead.
func (*Packet) Descriptor() ([]byte, []int) {
	return file_packages_network_warpsync_proto_message_proto_rawDescGZIP(), []int{0}
}

func (m *Packet) GetBody() isPacket_Body {
	if m != nil {
		return m.Body
	}
	return nil
}

func (x *Packet) GetEpochBlocksStart() *EpochBlocksStart {
	if x, ok := x.GetBody().(*Packet_EpochBlocksStart); ok {
		return x.EpochBlocksStart
	}
	return nil
}

func (x *Packet) GetEpochBlocksBatch() *EpochBlocksBatch {
	if x, ok := x.GetBody().(*Packet_EpochBlocksBatch); ok {
		return x.EpochBlocksBatch
	}
	return nil
}

func (x *Packet) GetEpochBlocksEnd() *EpochBlocksEnd {
	if x, ok := x.GetBody().(*Packet_EpochBlocksEnd); ok {
		return x.EpochBlocksEnd
	}
	return nil
}

func (x *Packet) GetEpochBlocksRequest() *EpochBlocksRequest {
	if x, ok := x.GetBody().(*Packet_EpochBlocksRequest); ok {
		return x.EpochBlocksRequest
	}
	return nil
}

func (x *Packet) GetEpochCommitment() *EpochCommittment {
	if x, ok := x.GetBody().(*Packet_EpochCommitment); ok {
		return x.EpochCommitment
	}
	return nil
}

func (x *Packet) GetEpochCommitmentRequest() *EpochCommittmentRequest {
	if x, ok := x.GetBody().(*Packet_EpochCommitmentRequest); ok {
		return x.EpochCommitmentRequest
	}
	return nil
}

func (x *Packet) GetNegotiation() *Negotiation {
	if x, ok := x.GetBody().(*Packet_Negotiation); ok {
		return x.Negotiation
	}
	return nil
}

type isPacket_Body interface {
	isPacket_Body()
}

type Packet_EpochBlocksStart struct {
	EpochBlocksStart *EpochBlocksStart `protobuf:"bytes,1,opt,name=epochBlocksStart,proto3,oneof"`
}

type Packet_EpochBlocksBatch struct {
	EpochBlocksBatch *EpochBlocksBatch `protobuf:"bytes,2,opt,name=epochBlocksBatch,proto3,oneof"`
}

type Packet_EpochBlocksEnd struct {
	EpochBlocksEnd *EpochBlocksEnd `protobuf:"bytes,3,opt,name=epochBlocksEnd,proto3,oneof"`
}

type Packet_EpochBlocksRequest struct {
	EpochBlocksRequest *EpochBlocksRequest `protobuf:"bytes,4,opt,name=epochBlocksRequest,proto3,oneof"`
}

type Packet_EpochCommitment struct {
	EpochCommitment *EpochCommittment `protobuf:"bytes,5,opt,name=epochCommitment,proto3,oneof"`
}

type Packet_EpochCommitmentRequest struct {
	EpochCommitmentRequest *EpochCommittmentRequest `protobuf:"bytes,6,opt,name=epochCommitmentRequest,proto3,oneof"`
}

type Packet_Negotiation struct {
	Negotiation *Negotiation `protobuf:"bytes,7,opt,name=negotiation,proto3,oneof"`
}

func (*Packet_EpochBlocksStart) isPacket_Body() {}

func (*Packet_EpochBlocksBatch) isPacket_Body() {}

func (*Packet_EpochBlocksEnd) isPacket_Body() {}

func (*Packet_EpochBlocksRequest) isPacket_Body() {}

func (*Packet_EpochCommitment) isPacket_Body() {}

func (*Packet_EpochCommitmentRequest) isPacket_Body() {}

func (*Packet_Negotiation) isPacket_Body() {}

type EpochBlocksStart struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	EI          int64  `protobuf:"varint,1,opt,name=Index,proto3" json:"Index,omitempty"`
	EC          []byte `protobuf:"bytes,2,opt,name=EC,proto3" json:"EC,omitempty"`
	BlocksCount int64  `protobuf:"varint,3,opt,name=blocksCount,proto3" json:"blocksCount,omitempty"`
}

func (x *EpochBlocksStart) Reset() {
	*x = EpochBlocksStart{}
	if protoimpl.UnsafeEnabled {
		mi := &file_packages_network_warpsync_proto_message_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *EpochBlocksStart) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*EpochBlocksStart) ProtoMessage() {}

func (x *EpochBlocksStart) ProtoReflect() protoreflect.Message {
	mi := &file_packages_network_warpsync_proto_message_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use EpochBlocksStart.ProtoReflect.Descriptor instead.
func (*EpochBlocksStart) Descriptor() ([]byte, []int) {
	return file_packages_network_warpsync_proto_message_proto_rawDescGZIP(), []int{1}
}

func (x *EpochBlocksStart) GetEI() int64 {
	if x != nil {
		return x.EI
	}
	return 0
}

func (x *EpochBlocksStart) GetEC() []byte {
	if x != nil {
		return x.EC
	}
	return nil
}

func (x *EpochBlocksStart) GetBlocksCount() int64 {
	if x != nil {
		return x.BlocksCount
	}
	return 0
}

type EpochBlocksBatch struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	EI     int64    `protobuf:"varint,1,opt,name=Index,proto3" json:"Index,omitempty"`
	EC     []byte   `protobuf:"bytes,2,opt,name=EC,proto3" json:"EC,omitempty"`
	Blocks [][]byte `protobuf:"bytes,3,rep,name=blocks,proto3" json:"blocks,omitempty"`
}

func (x *EpochBlocksBatch) Reset() {
	*x = EpochBlocksBatch{}
	if protoimpl.UnsafeEnabled {
		mi := &file_packages_network_warpsync_proto_message_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *EpochBlocksBatch) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*EpochBlocksBatch) ProtoMessage() {}

func (x *EpochBlocksBatch) ProtoReflect() protoreflect.Message {
	mi := &file_packages_network_warpsync_proto_message_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use EpochBlocksBatch.ProtoReflect.Descriptor instead.
func (*EpochBlocksBatch) Descriptor() ([]byte, []int) {
	return file_packages_network_warpsync_proto_message_proto_rawDescGZIP(), []int{2}
}

func (x *EpochBlocksBatch) GetEI() int64 {
	if x != nil {
		return x.EI
	}
	return 0
}

func (x *EpochBlocksBatch) GetEC() []byte {
	if x != nil {
		return x.EC
	}
	return nil
}

func (x *EpochBlocksBatch) GetBlocks() [][]byte {
	if x != nil {
		return x.Blocks
	}
	return nil
}

type EpochBlocksEnd struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	EI                int64  `protobuf:"varint,1,opt,name=Index,proto3" json:"Index,omitempty"`
	EC                []byte `protobuf:"bytes,2,opt,name=EC,proto3" json:"EC,omitempty"`
	StateMutationRoot []byte `protobuf:"bytes,3,opt,name=stateMutationRoot,proto3" json:"stateMutationRoot,omitempty"`
	StateRoot         []byte `protobuf:"bytes,4,opt,name=stateRoot,proto3" json:"stateRoot,omitempty"`
	ManaRoot          []byte `protobuf:"bytes,5,opt,name=manaRoot,proto3" json:"manaRoot,omitempty"`
}

func (x *EpochBlocksEnd) Reset() {
	*x = EpochBlocksEnd{}
	if protoimpl.UnsafeEnabled {
		mi := &file_packages_network_warpsync_proto_message_proto_msgTypes[3]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *EpochBlocksEnd) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*EpochBlocksEnd) ProtoMessage() {}

func (x *EpochBlocksEnd) ProtoReflect() protoreflect.Message {
	mi := &file_packages_network_warpsync_proto_message_proto_msgTypes[3]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use EpochBlocksEnd.ProtoReflect.Descriptor instead.
func (*EpochBlocksEnd) Descriptor() ([]byte, []int) {
	return file_packages_network_warpsync_proto_message_proto_rawDescGZIP(), []int{3}
}

func (x *EpochBlocksEnd) GetEI() int64 {
	if x != nil {
		return x.EI
	}
	return 0
}

func (x *EpochBlocksEnd) GetEC() []byte {
	if x != nil {
		return x.EC
	}
	return nil
}

func (x *EpochBlocksEnd) GetStateMutationRoot() []byte {
	if x != nil {
		return x.StateMutationRoot
	}
	return nil
}

func (x *EpochBlocksEnd) GetStateRoot() []byte {
	if x != nil {
		return x.StateRoot
	}
	return nil
}

func (x *EpochBlocksEnd) GetManaRoot() []byte {
	if x != nil {
		return x.ManaRoot
	}
	return nil
}

type EpochBlocksRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	EI int64  `protobuf:"varint,1,opt,name=Index,proto3" json:"Index,omitempty"`
	EC []byte `protobuf:"bytes,2,opt,name=EC,proto3" json:"EC,omitempty"`
}

func (x *EpochBlocksRequest) Reset() {
	*x = EpochBlocksRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_packages_network_warpsync_proto_message_proto_msgTypes[4]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *EpochBlocksRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*EpochBlocksRequest) ProtoMessage() {}

func (x *EpochBlocksRequest) ProtoReflect() protoreflect.Message {
	mi := &file_packages_network_warpsync_proto_message_proto_msgTypes[4]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use EpochBlocksRequest.ProtoReflect.Descriptor instead.
func (*EpochBlocksRequest) Descriptor() ([]byte, []int) {
	return file_packages_network_warpsync_proto_message_proto_rawDescGZIP(), []int{4}
}

func (x *EpochBlocksRequest) GetEI() int64 {
	if x != nil {
		return x.EI
	}
	return 0
}

func (x *EpochBlocksRequest) GetEC() []byte {
	if x != nil {
		return x.EC
	}
	return nil
}

type EpochCommittment struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	EI     int64  `protobuf:"varint,1,opt,name=Index,proto3" json:"Index,omitempty"`
	PrevEC []byte `protobuf:"bytes,2,opt,name=prevEC,proto3" json:"prevEC,omitempty"`
	ECR    []byte `protobuf:"bytes,3,opt,name=RootsID,proto3" json:"RootsID,omitempty"`
}

func (x *EpochCommittment) Reset() {
	*x = EpochCommittment{}
	if protoimpl.UnsafeEnabled {
		mi := &file_packages_network_warpsync_proto_message_proto_msgTypes[5]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *EpochCommittment) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*EpochCommittment) ProtoMessage() {}

func (x *EpochCommittment) ProtoReflect() protoreflect.Message {
	mi := &file_packages_network_warpsync_proto_message_proto_msgTypes[5]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use EpochCommittment.ProtoReflect.Descriptor instead.
func (*EpochCommittment) Descriptor() ([]byte, []int) {
	return file_packages_network_warpsync_proto_message_proto_rawDescGZIP(), []int{5}
}

func (x *EpochCommittment) GetEI() int64 {
	if x != nil {
		return x.EI
	}
	return 0
}

func (x *EpochCommittment) GetPrevEC() []byte {
	if x != nil {
		return x.PrevEC
	}
	return nil
}

func (x *EpochCommittment) GetECR() []byte {
	if x != nil {
		return x.ECR
	}
	return nil
}

type EpochCommittmentRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	EI int64 `protobuf:"varint,1,opt,name=Index,proto3" json:"Index,omitempty"`
}

func (x *EpochCommittmentRequest) Reset() {
	*x = EpochCommittmentRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_packages_network_warpsync_proto_message_proto_msgTypes[6]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *EpochCommittmentRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*EpochCommittmentRequest) ProtoMessage() {}

func (x *EpochCommittmentRequest) ProtoReflect() protoreflect.Message {
	mi := &file_packages_network_warpsync_proto_message_proto_msgTypes[6]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use EpochCommittmentRequest.ProtoReflect.Descriptor instead.
func (*EpochCommittmentRequest) Descriptor() ([]byte, []int) {
	return file_packages_network_warpsync_proto_message_proto_rawDescGZIP(), []int{6}
}

func (x *EpochCommittmentRequest) GetEI() int64 {
	if x != nil {
		return x.EI
	}
	return 0
}

type Negotiation struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields
}

func (x *Negotiation) Reset() {
	*x = Negotiation{}
	if protoimpl.UnsafeEnabled {
		mi := &file_packages_network_warpsync_proto_message_proto_msgTypes[7]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Negotiation) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Negotiation) ProtoMessage() {}

func (x *Negotiation) ProtoReflect() protoreflect.Message {
	mi := &file_packages_network_warpsync_proto_message_proto_msgTypes[7]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Negotiation.ProtoReflect.Descriptor instead.
func (*Negotiation) Descriptor() ([]byte, []int) {
	return file_packages_network_warpsync_proto_message_proto_rawDescGZIP(), []int{7}
}

var File_packages_network_warpsync_proto_message_proto protoreflect.FileDescriptor

var file_packages_network_warpsync_proto_message_proto_rawDesc = []byte{
	0x0a, 0x2d, 0x70, 0x61, 0x63, 0x6b, 0x61, 0x67, 0x65, 0x73, 0x2f, 0x6e, 0x65, 0x74, 0x77, 0x6f,
	0x72, 0x6b, 0x2f, 0x77, 0x61, 0x72, 0x70, 0x73, 0x79, 0x6e, 0x63, 0x2f, 0x70, 0x72, 0x6f, 0x74,
	0x6f, 0x2f, 0x6d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12,
	0x0d, 0x77, 0x61, 0x72, 0x70, 0x73, 0x79, 0x6e, 0x63, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x22, 0xbb,
	0x04, 0x0a, 0x06, 0x50, 0x61, 0x63, 0x6b, 0x65, 0x74, 0x12, 0x4d, 0x0a, 0x10, 0x65, 0x70, 0x6f,
	0x63, 0x68, 0x42, 0x6c, 0x6f, 0x63, 0x6b, 0x73, 0x53, 0x74, 0x61, 0x72, 0x74, 0x18, 0x01, 0x20,
	0x01, 0x28, 0x0b, 0x32, 0x1f, 0x2e, 0x77, 0x61, 0x72, 0x70, 0x73, 0x79, 0x6e, 0x63, 0x70, 0x72,
	0x6f, 0x74, 0x6f, 0x2e, 0x45, 0x70, 0x6f, 0x63, 0x68, 0x42, 0x6c, 0x6f, 0x63, 0x6b, 0x73, 0x53,
	0x74, 0x61, 0x72, 0x74, 0x48, 0x00, 0x52, 0x10, 0x65, 0x70, 0x6f, 0x63, 0x68, 0x42, 0x6c, 0x6f,
	0x63, 0x6b, 0x73, 0x53, 0x74, 0x61, 0x72, 0x74, 0x12, 0x4d, 0x0a, 0x10, 0x65, 0x70, 0x6f, 0x63,
	0x68, 0x42, 0x6c, 0x6f, 0x63, 0x6b, 0x73, 0x42, 0x61, 0x74, 0x63, 0x68, 0x18, 0x02, 0x20, 0x01,
	0x28, 0x0b, 0x32, 0x1f, 0x2e, 0x77, 0x61, 0x72, 0x70, 0x73, 0x79, 0x6e, 0x63, 0x70, 0x72, 0x6f,
	0x74, 0x6f, 0x2e, 0x45, 0x70, 0x6f, 0x63, 0x68, 0x42, 0x6c, 0x6f, 0x63, 0x6b, 0x73, 0x42, 0x61,
	0x74, 0x63, 0x68, 0x48, 0x00, 0x52, 0x10, 0x65, 0x70, 0x6f, 0x63, 0x68, 0x42, 0x6c, 0x6f, 0x63,
	0x6b, 0x73, 0x42, 0x61, 0x74, 0x63, 0x68, 0x12, 0x47, 0x0a, 0x0e, 0x65, 0x70, 0x6f, 0x63, 0x68,
	0x42, 0x6c, 0x6f, 0x63, 0x6b, 0x73, 0x45, 0x6e, 0x64, 0x18, 0x03, 0x20, 0x01, 0x28, 0x0b, 0x32,
	0x1d, 0x2e, 0x77, 0x61, 0x72, 0x70, 0x73, 0x79, 0x6e, 0x63, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2e,
	0x45, 0x70, 0x6f, 0x63, 0x68, 0x42, 0x6c, 0x6f, 0x63, 0x6b, 0x73, 0x45, 0x6e, 0x64, 0x48, 0x00,
	0x52, 0x0e, 0x65, 0x70, 0x6f, 0x63, 0x68, 0x42, 0x6c, 0x6f, 0x63, 0x6b, 0x73, 0x45, 0x6e, 0x64,
	0x12, 0x53, 0x0a, 0x12, 0x65, 0x70, 0x6f, 0x63, 0x68, 0x42, 0x6c, 0x6f, 0x63, 0x6b, 0x73, 0x52,
	0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x18, 0x04, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x21, 0x2e, 0x77,
	0x61, 0x72, 0x70, 0x73, 0x79, 0x6e, 0x63, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2e, 0x45, 0x70, 0x6f,
	0x63, 0x68, 0x42, 0x6c, 0x6f, 0x63, 0x6b, 0x73, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x48,
	0x00, 0x52, 0x12, 0x65, 0x70, 0x6f, 0x63, 0x68, 0x42, 0x6c, 0x6f, 0x63, 0x6b, 0x73, 0x52, 0x65,
	0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x4b, 0x0a, 0x0f, 0x65, 0x70, 0x6f, 0x63, 0x68, 0x43, 0x6f,
	0x6d, 0x6d, 0x69, 0x74, 0x6d, 0x65, 0x6e, 0x74, 0x18, 0x05, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x1f,
	0x2e, 0x77, 0x61, 0x72, 0x70, 0x73, 0x79, 0x6e, 0x63, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2e, 0x45,
	0x70, 0x6f, 0x63, 0x68, 0x43, 0x6f, 0x6d, 0x6d, 0x69, 0x74, 0x74, 0x6d, 0x65, 0x6e, 0x74, 0x48,
	0x00, 0x52, 0x0f, 0x65, 0x70, 0x6f, 0x63, 0x68, 0x43, 0x6f, 0x6d, 0x6d, 0x69, 0x74, 0x6d, 0x65,
	0x6e, 0x74, 0x12, 0x60, 0x0a, 0x16, 0x65, 0x70, 0x6f, 0x63, 0x68, 0x43, 0x6f, 0x6d, 0x6d, 0x69,
	0x74, 0x6d, 0x65, 0x6e, 0x74, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x18, 0x06, 0x20, 0x01,
	0x28, 0x0b, 0x32, 0x26, 0x2e, 0x77, 0x61, 0x72, 0x70, 0x73, 0x79, 0x6e, 0x63, 0x70, 0x72, 0x6f,
	0x74, 0x6f, 0x2e, 0x45, 0x70, 0x6f, 0x63, 0x68, 0x43, 0x6f, 0x6d, 0x6d, 0x69, 0x74, 0x74, 0x6d,
	0x65, 0x6e, 0x74, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x48, 0x00, 0x52, 0x16, 0x65, 0x70,
	0x6f, 0x63, 0x68, 0x43, 0x6f, 0x6d, 0x6d, 0x69, 0x74, 0x6d, 0x65, 0x6e, 0x74, 0x52, 0x65, 0x71,
	0x75, 0x65, 0x73, 0x74, 0x12, 0x3e, 0x0a, 0x0b, 0x6e, 0x65, 0x67, 0x6f, 0x74, 0x69, 0x61, 0x74,
	0x69, 0x6f, 0x6e, 0x18, 0x07, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x1a, 0x2e, 0x77, 0x61, 0x72, 0x70,
	0x73, 0x79, 0x6e, 0x63, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2e, 0x4e, 0x65, 0x67, 0x6f, 0x74, 0x69,
	0x61, 0x74, 0x69, 0x6f, 0x6e, 0x48, 0x00, 0x52, 0x0b, 0x6e, 0x65, 0x67, 0x6f, 0x74, 0x69, 0x61,
	0x74, 0x69, 0x6f, 0x6e, 0x42, 0x06, 0x0a, 0x04, 0x62, 0x6f, 0x64, 0x79, 0x22, 0x54, 0x0a, 0x10,
	0x45, 0x70, 0x6f, 0x63, 0x68, 0x42, 0x6c, 0x6f, 0x63, 0x6b, 0x73, 0x53, 0x74, 0x61, 0x72, 0x74,
	0x12, 0x0e, 0x0a, 0x02, 0x45, 0x49, 0x18, 0x01, 0x20, 0x01, 0x28, 0x03, 0x52, 0x02, 0x45, 0x49,
	0x12, 0x0e, 0x0a, 0x02, 0x45, 0x43, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0c, 0x52, 0x02, 0x45, 0x43,
	0x12, 0x20, 0x0a, 0x0b, 0x62, 0x6c, 0x6f, 0x63, 0x6b, 0x73, 0x43, 0x6f, 0x75, 0x6e, 0x74, 0x18,
	0x03, 0x20, 0x01, 0x28, 0x03, 0x52, 0x0b, 0x62, 0x6c, 0x6f, 0x63, 0x6b, 0x73, 0x43, 0x6f, 0x75,
	0x6e, 0x74, 0x22, 0x4a, 0x0a, 0x10, 0x45, 0x70, 0x6f, 0x63, 0x68, 0x42, 0x6c, 0x6f, 0x63, 0x6b,
	0x73, 0x42, 0x61, 0x74, 0x63, 0x68, 0x12, 0x0e, 0x0a, 0x02, 0x45, 0x49, 0x18, 0x01, 0x20, 0x01,
	0x28, 0x03, 0x52, 0x02, 0x45, 0x49, 0x12, 0x0e, 0x0a, 0x02, 0x45, 0x43, 0x18, 0x02, 0x20, 0x01,
	0x28, 0x0c, 0x52, 0x02, 0x45, 0x43, 0x12, 0x16, 0x0a, 0x06, 0x62, 0x6c, 0x6f, 0x63, 0x6b, 0x73,
	0x18, 0x03, 0x20, 0x03, 0x28, 0x0c, 0x52, 0x06, 0x62, 0x6c, 0x6f, 0x63, 0x6b, 0x73, 0x22, 0x98,
	0x01, 0x0a, 0x0e, 0x45, 0x70, 0x6f, 0x63, 0x68, 0x42, 0x6c, 0x6f, 0x63, 0x6b, 0x73, 0x45, 0x6e,
	0x64, 0x12, 0x0e, 0x0a, 0x02, 0x45, 0x49, 0x18, 0x01, 0x20, 0x01, 0x28, 0x03, 0x52, 0x02, 0x45,
	0x49, 0x12, 0x0e, 0x0a, 0x02, 0x45, 0x43, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0c, 0x52, 0x02, 0x45,
	0x43, 0x12, 0x2c, 0x0a, 0x11, 0x73, 0x74, 0x61, 0x74, 0x65, 0x4d, 0x75, 0x74, 0x61, 0x74, 0x69,
	0x6f, 0x6e, 0x52, 0x6f, 0x6f, 0x74, 0x18, 0x03, 0x20, 0x01, 0x28, 0x0c, 0x52, 0x11, 0x73, 0x74,
	0x61, 0x74, 0x65, 0x4d, 0x75, 0x74, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x52, 0x6f, 0x6f, 0x74, 0x12,
	0x1c, 0x0a, 0x09, 0x73, 0x74, 0x61, 0x74, 0x65, 0x52, 0x6f, 0x6f, 0x74, 0x18, 0x04, 0x20, 0x01,
	0x28, 0x0c, 0x52, 0x09, 0x73, 0x74, 0x61, 0x74, 0x65, 0x52, 0x6f, 0x6f, 0x74, 0x12, 0x1a, 0x0a,
	0x08, 0x6d, 0x61, 0x6e, 0x61, 0x52, 0x6f, 0x6f, 0x74, 0x18, 0x05, 0x20, 0x01, 0x28, 0x0c, 0x52,
	0x08, 0x6d, 0x61, 0x6e, 0x61, 0x52, 0x6f, 0x6f, 0x74, 0x22, 0x34, 0x0a, 0x12, 0x45, 0x70, 0x6f,
	0x63, 0x68, 0x42, 0x6c, 0x6f, 0x63, 0x6b, 0x73, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12,
	0x0e, 0x0a, 0x02, 0x45, 0x49, 0x18, 0x01, 0x20, 0x01, 0x28, 0x03, 0x52, 0x02, 0x45, 0x49, 0x12,
	0x0e, 0x0a, 0x02, 0x45, 0x43, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0c, 0x52, 0x02, 0x45, 0x43, 0x22,
	0x4c, 0x0a, 0x10, 0x45, 0x70, 0x6f, 0x63, 0x68, 0x43, 0x6f, 0x6d, 0x6d, 0x69, 0x74, 0x74, 0x6d,
	0x65, 0x6e, 0x74, 0x12, 0x0e, 0x0a, 0x02, 0x45, 0x49, 0x18, 0x01, 0x20, 0x01, 0x28, 0x03, 0x52,
	0x02, 0x45, 0x49, 0x12, 0x16, 0x0a, 0x06, 0x70, 0x72, 0x65, 0x76, 0x45, 0x43, 0x18, 0x02, 0x20,
	0x01, 0x28, 0x0c, 0x52, 0x06, 0x70, 0x72, 0x65, 0x76, 0x45, 0x43, 0x12, 0x10, 0x0a, 0x03, 0x45,
	0x43, 0x52, 0x18, 0x03, 0x20, 0x01, 0x28, 0x0c, 0x52, 0x03, 0x45, 0x43, 0x52, 0x22, 0x29, 0x0a,
	0x17, 0x45, 0x70, 0x6f, 0x63, 0x68, 0x43, 0x6f, 0x6d, 0x6d, 0x69, 0x74, 0x74, 0x6d, 0x65, 0x6e,
	0x74, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x0e, 0x0a, 0x02, 0x45, 0x49, 0x18, 0x01,
	0x20, 0x01, 0x28, 0x03, 0x52, 0x02, 0x45, 0x49, 0x22, 0x0d, 0x0a, 0x0b, 0x4e, 0x65, 0x67, 0x6f,
	0x74, 0x69, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x42, 0x41, 0x5a, 0x3f, 0x67, 0x69, 0x74, 0x68, 0x75,
	0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x69, 0x6f, 0x74, 0x61, 0x6c, 0x65, 0x64, 0x67, 0x65, 0x72,
	0x2f, 0x67, 0x6f, 0x73, 0x68, 0x69, 0x6d, 0x6d, 0x65, 0x72, 0x2f, 0x70, 0x61, 0x63, 0x6b, 0x61,
	0x67, 0x65, 0x73, 0x2f, 0x6e, 0x65, 0x74, 0x77, 0x6f, 0x72, 0x6b, 0x2f, 0x77, 0x61, 0x72, 0x70,
	0x73, 0x79, 0x6e, 0x63, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74,
	0x6f, 0x33,
}

var (
	file_packages_network_warpsync_proto_message_proto_rawDescOnce sync.Once
	file_packages_network_warpsync_proto_message_proto_rawDescData = file_packages_network_warpsync_proto_message_proto_rawDesc
)

func file_packages_network_warpsync_proto_message_proto_rawDescGZIP() []byte {
	file_packages_network_warpsync_proto_message_proto_rawDescOnce.Do(func() {
		file_packages_network_warpsync_proto_message_proto_rawDescData = protoimpl.X.CompressGZIP(file_packages_network_warpsync_proto_message_proto_rawDescData)
	})
	return file_packages_network_warpsync_proto_message_proto_rawDescData
}

var file_packages_network_warpsync_proto_message_proto_msgTypes = make([]protoimpl.MessageInfo, 8)
var file_packages_network_warpsync_proto_message_proto_goTypes = []interface{}{
	(*Packet)(nil),                  // 0: warpsyncproto.Packet
	(*EpochBlocksStart)(nil),        // 1: warpsyncproto.EpochBlocksStart
	(*EpochBlocksBatch)(nil),        // 2: warpsyncproto.EpochBlocksBatch
	(*EpochBlocksEnd)(nil),          // 3: warpsyncproto.EpochBlocksEnd
	(*EpochBlocksRequest)(nil),      // 4: warpsyncproto.EpochBlocksRequest
	(*EpochCommittment)(nil),        // 5: warpsyncproto.EpochCommittment
	(*EpochCommittmentRequest)(nil), // 6: warpsyncproto.EpochCommittmentRequest
	(*Negotiation)(nil),             // 7: warpsyncproto.Negotiation
}
var file_packages_network_warpsync_proto_message_proto_depIdxs = []int32{
	1, // 0: warpsyncproto.Packet.epochBlocksStart:type_name -> warpsyncproto.EpochBlocksStart
	2, // 1: warpsyncproto.Packet.epochBlocksBatch:type_name -> warpsyncproto.EpochBlocksBatch
	3, // 2: warpsyncproto.Packet.epochBlocksEnd:type_name -> warpsyncproto.EpochBlocksEnd
	4, // 3: warpsyncproto.Packet.epochBlocksRequest:type_name -> warpsyncproto.EpochBlocksRequest
	5, // 4: warpsyncproto.Packet.epochCommitment:type_name -> warpsyncproto.EpochCommittment
	6, // 5: warpsyncproto.Packet.epochCommitmentRequest:type_name -> warpsyncproto.EpochCommittmentRequest
	7, // 6: warpsyncproto.Packet.negotiation:type_name -> warpsyncproto.Negotiation
	7, // [7:7] is the sub-list for method output_type
	7, // [7:7] is the sub-list for method input_type
	7, // [7:7] is the sub-list for extension type_name
	7, // [7:7] is the sub-list for extension extendee
	0, // [0:7] is the sub-list for field type_name
}

func init() { file_packages_network_warpsync_proto_message_proto_init() }
func file_packages_network_warpsync_proto_message_proto_init() {
	if File_packages_network_warpsync_proto_message_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_packages_network_warpsync_proto_message_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Packet); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_packages_network_warpsync_proto_message_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*EpochBlocksStart); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_packages_network_warpsync_proto_message_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*EpochBlocksBatch); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_packages_network_warpsync_proto_message_proto_msgTypes[3].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*EpochBlocksEnd); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_packages_network_warpsync_proto_message_proto_msgTypes[4].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*EpochBlocksRequest); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_packages_network_warpsync_proto_message_proto_msgTypes[5].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*EpochCommittment); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_packages_network_warpsync_proto_message_proto_msgTypes[6].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*EpochCommittmentRequest); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_packages_network_warpsync_proto_message_proto_msgTypes[7].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Negotiation); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
	}
	file_packages_network_warpsync_proto_message_proto_msgTypes[0].OneofWrappers = []interface{}{
		(*Packet_EpochBlocksStart)(nil),
		(*Packet_EpochBlocksBatch)(nil),
		(*Packet_EpochBlocksEnd)(nil),
		(*Packet_EpochBlocksRequest)(nil),
		(*Packet_EpochCommitment)(nil),
		(*Packet_EpochCommitmentRequest)(nil),
		(*Packet_Negotiation)(nil),
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_packages_network_warpsync_proto_message_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   8,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_packages_network_warpsync_proto_message_proto_goTypes,
		DependencyIndexes: file_packages_network_warpsync_proto_message_proto_depIdxs,
		MessageInfos:      file_packages_network_warpsync_proto_message_proto_msgTypes,
	}.Build()
	File_packages_network_warpsync_proto_message_proto = out.File
	file_packages_network_warpsync_proto_message_proto_rawDesc = nil
	file_packages_network_warpsync_proto_message_proto_goTypes = nil
	file_packages_network_warpsync_proto_message_proto_depIdxs = nil
}
