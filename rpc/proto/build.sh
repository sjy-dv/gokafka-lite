-- drpc

protoc -I=. ./v1/actor.proto --go_out=. --go-vtproto_out=. --go_opt=paths=source_relative --go-drpc_out=. --go-drpc_opt=protolib=github.com/planetscale/vtprotobuf/codec/drpc --go-drpc_opt=paths=source_relative --proto_path=. ./v1/*.proto

-- grpc

protoc --go-grpc_opt=require_unimplemented_servers=false --go-grpc_out=./ --proto_path=./ ./v1/*.proto && protoc --go_out=./ --proto_path=./ ./v1/*.proto
