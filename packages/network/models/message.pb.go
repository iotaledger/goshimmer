// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.28.1
// 	protoc        v3.6.1
// source: packages/network/models/message.proto

package models

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
	//	*Packet_Block
	//	*Packet_BlockRequest
	//	*Packet_EpochCommitment
	//	*Packet_EpochCommitmentRequest
	Body isPacket_Body `protobuf_oneof:"body"`
}

func (x *Packet) Reset() {
	*x = Packet{}
	if protoimpl.UnsafeEnabled {
		mi := &file_packages_network_models_message_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Packet) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Packet) ProtoMessage() {}

func (x *Packet) ProtoReflect() protoreflect.Message {
	mi := &file_packages_network_models_message_proto_msgTypes[0]
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
	return file_packages_network_models_message_proto_rawDescGZIP(), []int{0}
}

func (m *Packet) GetBody() isPacket_Body {
	if m != nil {
		return m.Body
	}
	return nil
}

func (x *Packet) GetBlock() *Block {
	if x, ok := x.GetBody().(*Packet_Block); ok {
		return x.Block
	}
	return nil
}

func (x *Packet) GetBlockRequest() *BlockRequest {
	if x, ok := x.GetBody().(*Packet_BlockRequest); ok {
		return x.BlockRequest
	}
	return nil
}

func (x *Packet) GetEpochCommitment() *EpochCommitment {
	if x, ok := x.GetBody().(*Packet_EpochCommitment); ok {
		return x.EpochCommitment
	}
	return nil
}

func (x *Packet) GetEpochCommitmentRequest() *EpochCommitmentRequest {
	if x, ok := x.GetBody().(*Packet_EpochCommitmentRequest); ok {
		return x.EpochCommitmentRequest
	}
	return nil
}

type isPacket_Body interface {
	isPacket_Body()
}

type Packet_Block struct {
	Block *Block `protobuf:"bytes,1,opt,name=block,proto3,oneof"`
}

type Packet_BlockRequest struct {
	BlockRequest *BlockRequest `protobuf:"bytes,2,opt,name=blockRequest,proto3,oneof"`
}

type Packet_EpochCommitment struct {
	EpochCommitment *EpochCommitment `protobuf:"bytes,3,opt,name=epochCommitment,proto3,oneof"`
}

type Packet_EpochCommitmentRequest struct {
	EpochCommitmentRequest *EpochCommitmentRequest `protobuf:"bytes,4,opt,name=epochCommitmentRequest,proto3,oneof"`
}

func (*Packet_Block) isPacket_Body() {}

func (*Packet_BlockRequest) isPacket_Body() {}

func (*Packet_EpochCommitment) isPacket_Body() {}

func (*Packet_EpochCommitmentRequest) isPacket_Body() {}

type Block struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Bytes []byte `protobuf:"bytes,1,opt,name=bytes,proto3" json:"bytes,omitempty"`
}

func (x *Block) Reset() {
	*x = Block{}
	if protoimpl.UnsafeEnabled {
		mi := &file_packages_network_models_message_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Block) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Block) ProtoMessage() {}

func (x *Block) ProtoReflect() protoreflect.Message {
	mi := &file_packages_network_models_message_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Block.ProtoReflect.Descriptor instead.
func (*Block) Descriptor() ([]byte, []int) {
	return file_packages_network_models_message_proto_rawDescGZIP(), []int{1}
}

func (x *Block) GetBytes() []byte {
	if x != nil {
		return x.Bytes
	}
	return nil
}

type BlockRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Bytes []byte `protobuf:"bytes,1,opt,name=bytes,proto3" json:"bytes,omitempty"`
}

func (x *BlockRequest) Reset() {
	*x = BlockRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_packages_network_models_message_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *BlockRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*BlockRequest) ProtoMessage() {}

func (x *BlockRequest) ProtoReflect() protoreflect.Message {
	mi := &file_packages_network_models_message_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use BlockRequest.ProtoReflect.Descriptor instead.
func (*BlockRequest) Descriptor() ([]byte, []int) {
	return file_packages_network_models_message_proto_rawDescGZIP(), []int{2}
}

func (x *BlockRequest) GetBytes() []byte {
	if x != nil {
		return x.Bytes
	}
	return nil
}

type EpochCommitment struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Bytes []byte `protobuf:"bytes,1,opt,name=bytes,proto3" json:"bytes,omitempty"`
}

func (x *EpochCommitment) Reset() {
	*x = EpochCommitment{}
	if protoimpl.UnsafeEnabled {
		mi := &file_packages_network_models_message_proto_msgTypes[3]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *EpochCommitment) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*EpochCommitment) ProtoMessage() {}

func (x *EpochCommitment) ProtoReflect() protoreflect.Message {
	mi := &file_packages_network_models_message_proto_msgTypes[3]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use EpochCommitment.ProtoReflect.Descriptor instead.
func (*EpochCommitment) Descriptor() ([]byte, []int) {
	return file_packages_network_models_message_proto_rawDescGZIP(), []int{3}
}

func (x *EpochCommitment) GetBytes() []byte {
	if x != nil {
		return x.Bytes
	}
	return nil
}

type EpochCommitmentRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Bytes []byte `protobuf:"bytes,1,opt,name=bytes,proto3" json:"bytes,omitempty"`
}

func (x *EpochCommitmentRequest) Reset() {
	*x = EpochCommitmentRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_packages_network_models_message_proto_msgTypes[4]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *EpochCommitmentRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*EpochCommitmentRequest) ProtoMessage() {}

func (x *EpochCommitmentRequest) ProtoReflect() protoreflect.Message {
	mi := &file_packages_network_models_message_proto_msgTypes[4]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use EpochCommitmentRequest.ProtoReflect.Descriptor instead.
func (*EpochCommitmentRequest) Descriptor() ([]byte, []int) {
	return file_packages_network_models_message_proto_rawDescGZIP(), []int{4}
}

func (x *EpochCommitmentRequest) GetBytes() []byte {
	if x != nil {
		return x.Bytes
	}
	return nil
}

var File_packages_network_models_message_proto protoreflect.FileDescriptor

var file_packages_network_models_message_proto_rawDesc = []byte{
	0x0a, 0x25, 0x70, 0x61, 0x63, 0x6b, 0x61, 0x67, 0x65, 0x73, 0x2f, 0x6e, 0x65, 0x74, 0x77, 0x6f,
	0x72, 0x6b, 0x2f, 0x6d, 0x6f, 0x64, 0x65, 0x6c, 0x73, 0x2f, 0x6d, 0x65, 0x73, 0x73, 0x61, 0x67,
	0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x06, 0x6d, 0x6f, 0x64, 0x65, 0x6c, 0x73, 0x22,
	0x92, 0x02, 0x0a, 0x06, 0x50, 0x61, 0x63, 0x6b, 0x65, 0x74, 0x12, 0x25, 0x0a, 0x05, 0x62, 0x6c,
	0x6f, 0x63, 0x6b, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x0d, 0x2e, 0x6d, 0x6f, 0x64, 0x65,
	0x6c, 0x73, 0x2e, 0x42, 0x6c, 0x6f, 0x63, 0x6b, 0x48, 0x00, 0x52, 0x05, 0x62, 0x6c, 0x6f, 0x63,
	0x6b, 0x12, 0x3a, 0x0a, 0x0c, 0x62, 0x6c, 0x6f, 0x63, 0x6b, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73,
	0x74, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x14, 0x2e, 0x6d, 0x6f, 0x64, 0x65, 0x6c, 0x73,
	0x2e, 0x42, 0x6c, 0x6f, 0x63, 0x6b, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x48, 0x00, 0x52,
	0x0c, 0x62, 0x6c, 0x6f, 0x63, 0x6b, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x43, 0x0a,
	0x0f, 0x65, 0x70, 0x6f, 0x63, 0x68, 0x43, 0x6f, 0x6d, 0x6d, 0x69, 0x74, 0x6d, 0x65, 0x6e, 0x74,
	0x18, 0x03, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x17, 0x2e, 0x6d, 0x6f, 0x64, 0x65, 0x6c, 0x73, 0x2e,
	0x45, 0x70, 0x6f, 0x63, 0x68, 0x43, 0x6f, 0x6d, 0x6d, 0x69, 0x74, 0x6d, 0x65, 0x6e, 0x74, 0x48,
	0x00, 0x52, 0x0f, 0x65, 0x70, 0x6f, 0x63, 0x68, 0x43, 0x6f, 0x6d, 0x6d, 0x69, 0x74, 0x6d, 0x65,
	0x6e, 0x74, 0x12, 0x58, 0x0a, 0x16, 0x65, 0x70, 0x6f, 0x63, 0x68, 0x43, 0x6f, 0x6d, 0x6d, 0x69,
	0x74, 0x6d, 0x65, 0x6e, 0x74, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x18, 0x04, 0x20, 0x01,
	0x28, 0x0b, 0x32, 0x1e, 0x2e, 0x6d, 0x6f, 0x64, 0x65, 0x6c, 0x73, 0x2e, 0x45, 0x70, 0x6f, 0x63,
	0x68, 0x43, 0x6f, 0x6d, 0x6d, 0x69, 0x74, 0x6d, 0x65, 0x6e, 0x74, 0x52, 0x65, 0x71, 0x75, 0x65,
	0x73, 0x74, 0x48, 0x00, 0x52, 0x16, 0x65, 0x70, 0x6f, 0x63, 0x68, 0x43, 0x6f, 0x6d, 0x6d, 0x69,
	0x74, 0x6d, 0x65, 0x6e, 0x74, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x42, 0x06, 0x0a, 0x04,
	0x62, 0x6f, 0x64, 0x79, 0x22, 0x1d, 0x0a, 0x05, 0x42, 0x6c, 0x6f, 0x63, 0x6b, 0x12, 0x14, 0x0a,
	0x05, 0x62, 0x79, 0x74, 0x65, 0x73, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0c, 0x52, 0x05, 0x62, 0x79,
	0x74, 0x65, 0x73, 0x22, 0x24, 0x0a, 0x0c, 0x42, 0x6c, 0x6f, 0x63, 0x6b, 0x52, 0x65, 0x71, 0x75,
	0x65, 0x73, 0x74, 0x12, 0x14, 0x0a, 0x05, 0x62, 0x79, 0x74, 0x65, 0x73, 0x18, 0x01, 0x20, 0x01,
	0x28, 0x0c, 0x52, 0x05, 0x62, 0x79, 0x74, 0x65, 0x73, 0x22, 0x27, 0x0a, 0x0f, 0x45, 0x70, 0x6f,
	0x63, 0x68, 0x43, 0x6f, 0x6d, 0x6d, 0x69, 0x74, 0x6d, 0x65, 0x6e, 0x74, 0x12, 0x14, 0x0a, 0x05,
	0x62, 0x79, 0x74, 0x65, 0x73, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0c, 0x52, 0x05, 0x62, 0x79, 0x74,
	0x65, 0x73, 0x22, 0x2e, 0x0a, 0x16, 0x45, 0x70, 0x6f, 0x63, 0x68, 0x43, 0x6f, 0x6d, 0x6d, 0x69,
	0x74, 0x6d, 0x65, 0x6e, 0x74, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x14, 0x0a, 0x05,
	0x62, 0x79, 0x74, 0x65, 0x73, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0c, 0x52, 0x05, 0x62, 0x79, 0x74,
	0x65, 0x73, 0x42, 0x39, 0x5a, 0x37, 0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d,
	0x2f, 0x69, 0x6f, 0x74, 0x61, 0x6c, 0x65, 0x64, 0x67, 0x65, 0x72, 0x2f, 0x67, 0x6f, 0x73, 0x68,
	0x69, 0x6d, 0x6d, 0x65, 0x72, 0x2f, 0x70, 0x61, 0x63, 0x6b, 0x61, 0x67, 0x65, 0x73, 0x2f, 0x6e,
	0x65, 0x74, 0x77, 0x6f, 0x72, 0x6b, 0x2f, 0x6d, 0x6f, 0x64, 0x65, 0x6c, 0x73, 0x62, 0x06, 0x70,
	0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_packages_network_models_message_proto_rawDescOnce sync.Once
	file_packages_network_models_message_proto_rawDescData = file_packages_network_models_message_proto_rawDesc
)

func file_packages_network_models_message_proto_rawDescGZIP() []byte {
	file_packages_network_models_message_proto_rawDescOnce.Do(func() {
		file_packages_network_models_message_proto_rawDescData = protoimpl.X.CompressGZIP(file_packages_network_models_message_proto_rawDescData)
	})
	return file_packages_network_models_message_proto_rawDescData
}

var file_packages_network_models_message_proto_msgTypes = make([]protoimpl.MessageInfo, 5)
var file_packages_network_models_message_proto_goTypes = []interface{}{
	(*Packet)(nil),                 // 0: models.Packet
	(*Block)(nil),                  // 1: models.Block
	(*BlockRequest)(nil),           // 2: models.BlockRequest
	(*EpochCommitment)(nil),        // 3: models.EpochCommitment
	(*EpochCommitmentRequest)(nil), // 4: models.EpochCommitmentRequest
}
var file_packages_network_models_message_proto_depIdxs = []int32{
	1, // 0: models.Packet.block:type_name -> models.Block
	2, // 1: models.Packet.blockRequest:type_name -> models.BlockRequest
	3, // 2: models.Packet.epochCommitment:type_name -> models.EpochCommitment
	4, // 3: models.Packet.epochCommitmentRequest:type_name -> models.EpochCommitmentRequest
	4, // [4:4] is the sub-list for method output_type
	4, // [4:4] is the sub-list for method input_type
	4, // [4:4] is the sub-list for extension type_name
	4, // [4:4] is the sub-list for extension extendee
	0, // [0:4] is the sub-list for field type_name
}

func init() { file_packages_network_models_message_proto_init() }
func file_packages_network_models_message_proto_init() {
	if File_packages_network_models_message_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_packages_network_models_message_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
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
		file_packages_network_models_message_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Block); i {
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
		file_packages_network_models_message_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*BlockRequest); i {
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
		file_packages_network_models_message_proto_msgTypes[3].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*EpochCommitment); i {
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
		file_packages_network_models_message_proto_msgTypes[4].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*EpochCommitmentRequest); i {
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
	file_packages_network_models_message_proto_msgTypes[0].OneofWrappers = []interface{}{
		(*Packet_Block)(nil),
		(*Packet_BlockRequest)(nil),
		(*Packet_EpochCommitment)(nil),
		(*Packet_EpochCommitmentRequest)(nil),
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_packages_network_models_message_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   5,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_packages_network_models_message_proto_goTypes,
		DependencyIndexes: file_packages_network_models_message_proto_depIdxs,
		MessageInfos:      file_packages_network_models_message_proto_msgTypes,
	}.Build()
	File_packages_network_models_message_proto = out.File
	file_packages_network_models_message_proto_rawDesc = nil
	file_packages_network_models_message_proto_goTypes = nil
	file_packages_network_models_message_proto_depIdxs = nil
}
