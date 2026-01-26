package svc

import (
	"fmt"
	"path/filepath"

	"agent/config"
	"agent/internal/agent"

	"github.com/kardianos/service"
)

// Service 封装了服务操作
type Service struct {
	svc service.Service
	prg *program
}

// program 实现 service.Interface
type program struct {
	agent   *agent.Agent
	cfgPath string
	logger  service.Logger
}

// New 创建一个新的服务实例
func New(cfgPath string) (*Service, error) {
	// 确保使用绝对路径
	absCfgPath, err := filepath.Abs(cfgPath)
	if err != nil {
		// 如果无法获取绝对路径，尝试使用默认逻辑
		if cfgPath == "" {
			absCfgPath = config.GetConfigPath()
		} else {
			absCfgPath = cfgPath
		}
	} else if cfgPath == "" {
		absCfgPath = config.GetConfigPath()
	}

	svcConfig := &service.Config{
		Name:        "cloudsentinel-agent",
		DisplayName: "CloudSentinel Agent",
		Description: "CloudSentinel Agent - 云哨监控代理",
		Arguments:   []string{"run", "--config", absCfgPath},
	}

	prg := &program{
		cfgPath: absCfgPath,
	}

	s, err := service.New(prg, svcConfig)
	if err != nil {
		return nil, err
	}

	// 初始化日志
	logger, err := s.Logger(nil)
	if err != nil {
		return nil, err
	}
	prg.logger = logger

	return &Service{
		svc: s,
		prg: prg,
	}, nil
}

// Install 安装服务
func (s *Service) Install() error {
	return s.svc.Install()
}

// Uninstall 卸载服务
func (s *Service) Uninstall() error {
	return s.svc.Uninstall()
}

// Start 启动服务（通过服务管理器）
func (s *Service) Start() error {
	return s.svc.Start()
}

// Stop 停止服务（通过服务管理器）
func (s *Service) Stop() error {
	return s.svc.Stop()
}

// Restart 重启服务
func (s *Service) Restart() error {
	return s.svc.Restart()
}

// Run 运行服务（阻塞，用于 'agent run' 命令）
func (s *Service) Run() error {
	return s.svc.Run()
}

// Status 获取服务状态
func (s *Service) Status() (string, error) {
	status, err := s.svc.Status()
	if err != nil {
		return "unknown", err
	}
	switch status {
	case service.StatusRunning:
		return "running", nil
	case service.StatusStopped:
		return "stopped", nil
	default:
		return "unknown", nil
	}
}

// service.Interface implementation

func (p *program) Start(s service.Service) error {
	// 实际运行 Agent，非阻塞
	go p.run()
	return nil
}

func (p *program) run() {
	if p.logger != nil {
		p.logger.Info("Starting CloudSentinel Agent service...")
	}

	cfg, err := config.LoadConfigFromFile(p.cfgPath)
	if err != nil {
		if p.logger != nil {
			p.logger.Error(fmt.Sprintf("Failed to load config from %s: %v", p.cfgPath, err))
		}
		return
	}

	a, err := agent.NewAgent(cfg)
	if err != nil {
		if p.logger != nil {
			p.logger.Error(fmt.Sprintf("Failed to create agent: %v", err))
		}
		return
	}
	p.agent = a

	if err := p.agent.Start(); err != nil {
		if p.logger != nil {
			p.logger.Error(fmt.Sprintf("Failed to start agent: %v", err))
		}
		return
	}

	if p.logger != nil {
		p.logger.Info("CloudSentinel Agent started successfully")
	}
}

func (p *program) Stop(s service.Service) error {
	if p.logger != nil {
		p.logger.Info("Stopping CloudSentinel Agent service...")
	}
	if p.agent != nil {
		p.agent.Stop()
	}
	if p.logger != nil {
		p.logger.Info("CloudSentinel Agent stopped")
	}
	return nil
}
