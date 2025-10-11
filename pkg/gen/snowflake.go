package gen

import (
	"log"

	"github.com/bwmarrin/snowflake"
)

type SnowflakeNode struct {
	node *snowflake.Node
}

func NewSnowflakeNode() (*SnowflakeNode, error) {
	node, err := snowflake.NewNode(1) // nodeID 1, ganti sesuai config
	if err != nil {
		log.Fatal("failed to init snowflake node:", err)
		return nil, err
	}
	return &SnowflakeNode{node: node}, nil
}

func (s *SnowflakeNode) GenerateID() snowflake.ID {
	return s.node.Generate()
}
