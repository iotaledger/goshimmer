// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.27.1
// 	protoc        v3.12.4
// source: packages/network/gossip/gossipproto/message.proto

package gossipproto

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
	//	*Packet_Block
	//	*Packet_BlockRequest
	Body isPacket_Body `protobuf_oneof:"body"`
}

func (x *Packet) Reset() {
	*x = Packet{}
	if protoimpl.UnsafeEnabled {
		mi := &file_packages_network_gossip_gossipproto_message_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Packet) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Packet) ProtoMessage() {}

func (x *Packet) ProtoReflect() protoreflect.Message {
	mi := &file_packages_network_gossip_gossipproto_message_proto_msgTypes[0]
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
	return file_packages_network_gossip_gossipproto_message_proto_rawDescGZIP(), []int{0}
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

type isPacket_Body interface {
	isPacket_Body()
}

type Packet_Block struct {
	Block *Block `protobuf:"bytes,1,opt,name=block,proto3,oneof"`
}

type Packet_BlockRequest struct {
	BlockRequest *BlockRequest `protobuf:"bytes,2,opt,name=blockRequest,proto3,oneof"`
}

func (*Packet_Block) isPacket_Body() {}

func (*Packet_BlockRequest) isPacket_Body() {}

type Block struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Data []byte `protobuf:"bytes,1,opt,name=data,proto3" json:"data,omitempty"`
}

func (x *Block) Reset() {
	*x = Block{}
	if protoimpl.UnsafeEnabled {
		mi := &file_packages_network_gossip_gossipproto_message_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Block) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Block) ProtoMessage() {}

func (x *Block) ProtoReflect() protoreflect.Message {
	mi := &file_packages_network_gossip_gossipproto_message_proto_msgTypes[1]
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
	return file_packages_network_gossip_gossipproto_message_proto_rawDescGZIP(), []int{1}
}

func (x *Block) GetData() []byte {
	if x != nil {
		return x.Data
	}
	return nil
}

type BlockRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Id []byte `protobuf:"bytes,1,opt,name=id,proto3" json:"id,omitempty"`
}

func (x *BlockRequest) Reset() {
	*x = BlockRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_packages_network_gossip_gossipproto_message_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *BlockRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*BlockRequest) ProtoMessage() {}

func (x *BlockRequest) ProtoReflect() protoreflect.Message {
	mi := &file_packages_network_gossip_gossipproto_message_proto_msgTypes[2]
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
	return file_packages_network_gossip_gossipproto_message_proto_rawDescGZIP(), []int{2}
}

func (x *BlockRequest) GetId() []byte {
	if x != nil {
		return x.Id
	}
	return nil
}

var File_packages_network_gossip_gossipproto_message_proto protoreflect.FileDescriptor

var file_packages_network_gossip_gossipproto_message_proto_rawDesc = []byte{
	0x0a, 0x31, 0x70, 0x61, 0x63, 0x6b, 0x61, 0x67, 0x65, 0x73, 0x2f, 0x6e, 0x65, 0x74, 0x77, 0x6f,
	0x72, 0x6b, 0x2f, 0x67, 0x6f, 0x73, 0x73, 0x69, 0x70, 0x2f, 0x67, 0x6f, 0x73, 0x73, 0x69, 0x70,
	0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2f, 0x6d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x2e, 0x70, 0x72,
	0x6f, 0x74, 0x6f, 0x12, 0x0b, 0x67, 0x6f, 0x73, 0x73, 0x69, 0x70, 0x70, 0x72, 0x6f, 0x74, 0x6f,
	0x22, 0x7d, 0x0a, 0x06, 0x50, 0x61, 0x63, 0x6b, 0x65, 0x74, 0x12, 0x2a, 0x0a, 0x05, 0x62, 0x6c,
	0x6f, 0x63, 0x6b, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x12, 0x2e, 0x67, 0x6f, 0x73, 0x73,
	0x69, 0x70, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2e, 0x42, 0x6c, 0x6f, 0x63, 0x6b, 0x48, 0x00, 0x52,
	0x05, 0x62, 0x6c, 0x6f, 0x63, 0x6b, 0x12, 0x3f, 0x0a, 0x0c, 0x62, 0x6c, 0x6f, 0x63, 0x6b, 0x52,
	0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x19, 0x2e, 0x67,
	0x6f, 0x73, 0x73, 0x69, 0x70, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2e, 0x42, 0x6c, 0x6f, 0x63, 0x6b,
	0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x48, 0x00, 0x52, 0x0c, 0x62, 0x6c, 0x6f, 0x63, 0x6b,
	0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x42, 0x06, 0x0a, 0x04, 0x62, 0x6f, 0x64, 0x79, 0x22,
	0x1b, 0x0a, 0x05, 0x42, 0x6c, 0x6f, 0x63, 0x6b, 0x12, 0x12, 0x0a, 0x04, 0x64, 0x61, 0x74, 0x61,
	0x18, 0x01, 0x20, 0x01, 0x28, 0x0c, 0x52, 0x04, 0x64, 0x61, 0x74, 0x61, 0x22, 0x1e, 0x0a, 0x0c,
	0x42, 0x6c, 0x6f, 0x63, 0x6b, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x0e, 0x0a, 0x02,
	0x69, 0x64, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0c, 0x52, 0x02, 0x69, 0x64, 0x42, 0x3d, 0x5a, 0x3b,
	0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x69, 0x6f, 0x74, 0x61, 0x6c,
	0x65, 0x64, 0x67, 0x65, 0x72, 0x2f, 0x67, 0x6f, 0x73, 0x68, 0x69, 0x6d, 0x6d, 0x65, 0x72, 0x2f,
	0x70, 0x61, 0x63, 0x6b, 0x61, 0x67, 0x65, 0x73, 0x2f, 0x67, 0x6f, 0x73, 0x73, 0x69, 0x70, 0x2f,
	0x67, 0x6f, 0x73, 0x73, 0x69, 0x70, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x06, 0x70, 0x72, 0x6f,
	0x74, 0x6f, 0x33,
}

var (
	file_packages_network_gossip_gossipproto_message_proto_rawDescOnce sync.Once
	file_packages_network_gossip_gossipproto_message_proto_rawDescData = file_packages_network_gossip_gossipproto_message_proto_rawDesc
)

func file_packages_network_gossip_gossipproto_message_proto_rawDescGZIP() []byte {
	file_packages_network_gossip_gossipproto_message_proto_rawDescOnce.Do(func() {
		file_packages_network_gossip_gossipproto_message_proto_rawDescData = protoimpl.X.CompressGZIP(file_packages_network_gossip_gossipproto_message_proto_rawDescData)
	})
	return file_packages_network_gossip_gossipproto_message_proto_rawDescData
}

var file_packages_network_gossip_gossipproto_message_proto_msgTypes = make([]protoimpl.MessageInfo, 3)
var file_packages_network_gossip_gossipproto_message_proto_goTypes = []interface{}{
	(*Packet)(nil),       // 0: gossipproto.Packet
	(*Block)(nil),        // 1: gossipproto.Block
	(*BlockRequest)(nil), // 2: gossipproto.BlockRequest
}
var file_packages_network_gossip_gossipproto_message_proto_depIdxs = []int32{
	1, // 0: gossipproto.Packet.block:type_name -> gossipproto.Block
	2, // 1: gossipproto.Packet.blockRequest:type_name -> gossipproto.BlockRequest
	2, // [2:2] is the sub-list for method output_type
	2, // [2:2] is the sub-list for method input_type
	2, // [2:2] is the sub-list for extension type_name
	2, // [2:2] is the sub-list for extension extendee
	0, // [0:2] is the sub-list for field type_name
}

func init() { file_packages_network_gossip_gossipproto_message_proto_init() }
func file_packages_network_gossip_gossipproto_message_proto_init() {
	if File_packages_network_gossip_gossipproto_message_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_packages_network_gossip_gossipproto_message_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
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
		file_packages_network_gossip_gossipproto_message_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
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
		file_packages_network_gossip_gossipproto_message_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
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
	}
	file_packages_network_gossip_gossipproto_message_proto_msgTypes[0].OneofWrappers = []interface{}{
		(*Packet_Block)(nil),
		(*Packet_BlockRequest)(nil),
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_packages_network_gossip_gossipproto_message_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   3,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_packages_network_gossip_gossipproto_message_proto_goTypes,
		DependencyIndexes: file_packages_network_gossip_gossipproto_message_proto_depIdxs,
		MessageInfos:      file_packages_network_gossip_gossipproto_message_proto_msgTypes,
	}.Build()
	File_packages_network_gossip_gossipproto_message_proto = out.File
	file_packages_network_gossip_gossipproto_message_proto_rawDesc = nil
	file_packages_network_gossip_gossipproto_message_proto_goTypes = nil
	file_packages_network_gossip_gossipproto_message_proto_depIdxs = nil
}
