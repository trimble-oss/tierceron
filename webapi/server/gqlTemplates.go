package server

import (
	"bitbucket.org/dexterchaney/whoville/vault-helper/kv"
	pb "bitbucket.org/dexterchaney/whoville/webapi/rpc/apinator"
)

func (s *Server) getTemplateData() (*pb.TemplateData, error) {
	mod, err := kv.NewModifier(s.VaultToken, s.VaultAddr, s.CertPath)
	if err != nil {
		return nil, err
	}
	services := []*pb.TemplateData_Service{}
	servicePaths := getPaths(mod, "templates/")
	for _, servicePath := range servicePaths {
		files := []*pb.TemplateData_Service_File{}
		filePaths := getPaths(mod, servicePath)
		for _, filePath := range filePaths {
			kvs, err := mod.ReadData(filePath)
			var secrets []string
			if err != nil {
				return nil, err
			}
			for k := range kvs {
				secrets = append(secrets, k)
			}
			file := &pb.TemplateData_Service_File{Name: getPathEnd(filePath), Secrets: secrets}
			files = append(files, file)
		}
		service := &pb.TemplateData_Service{Name: getPathEnd(servicePath), Files: files}
		services = append(services, service)
	}
	return &pb.TemplateData{
		Services: services,
	}, nil
}
