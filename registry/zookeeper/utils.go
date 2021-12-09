package zookeeper

import (
	"emt/registry"
	"encoding/json"
	"fmt"
	"path"
	"strings"

	"github.com/samuel/go-zookeeper/zk"
)

func encode(s *registry.Service) ([]byte, error) {
	return json.Marshal(s)
}

func decode(ds []byte) (*registry.Service, error) {
	var s *registry.Service

	return s, json.Unmarshal(ds, &s)
}

func createPath(path string, data []byte, client *zk.Conn) error {
	exists, _, err := client.Exists(path)
	if err != nil {
		return fmt.Errorf("fail to find client exist %w", err)
	}

	if exists {
		return nil
	}

	name := "/"

	p := strings.Split(path, "/")

	for _, v := range p[1 : len(p)-1] {
		name += v
		e, _, _ := client.Exists(name)

		if !e {
			_, err = client.Create(name, []byte{}, int32(0), zk.WorldACL(zk.PermAll))
			if err != nil {
				return fmt.Errorf("failed to create client %w", err)
			}
		}

		name += "/"
	}

	_, err = client.Create(path, data, int32(0), zk.WorldACL(zk.PermAll))
	if err != nil {
		return fmt.Errorf("createPath err = %w", err)
	}

	return nil
}

func nodePath(s, id string) string {
	service := strings.ReplaceAll(s, "/", "-")

	if id != "" {
		node := strings.ReplaceAll(id, "/", "-")
		p := service + "_" + node

		return path.Join(DefaultProjectName, p)
	}

	return path.Join(DefaultProjectName, service)
}

func vaguePath(path string) string {
	index := strings.Index(path, "_")

	return path[:index]
}
