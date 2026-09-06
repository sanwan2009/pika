package handler

import (
	"github.com/go-orz/orz"
	"github.com/labstack/echo/v5"
	"github.com/pika-monitor/pika/internal/service"
	"go.uber.org/zap"
)

type AlertRuleHandler struct {
	logger       *zap.Logger
	ruleService  *service.AlertRuleService
	agentService *service.AgentService
}

func NewAlertRuleHandler(logger *zap.Logger, ruleService *service.AlertRuleService, agentService *service.AgentService) *AlertRuleHandler {
	return &AlertRuleHandler{
		logger:       logger,
		ruleService:  ruleService,
		agentService: agentService,
	}
}

func (h *AlertRuleHandler) List(c *echo.Context) error {
	keyword := c.QueryParam("keyword")
	enabled := c.QueryParam("enabled")

	pr := orz.GetPageRequest(c, "priority")

	builder := orz.NewPageBuilder(h.ruleService.AlertRuleRepo).
		PageRequest(pr).
		Keyword([]string{"name"}, keyword)

	if enabled == "true" {
		builder.Equal("enabled", "1")
	} else if enabled == "false" {
		builder.Equal("enabled", "0")
	}

	ctx := c.Request().Context()
	page, err := builder.Execute(ctx)
	if err != nil {
		return err
	}

	// 填充主机名称
	var agentIds []string
	for _, item := range page.Items {
		if len(item.AgentIds) > 0 {
			agentIds = append(agentIds, item.AgentIds...)
		}
	}
	if len(agentIds) > 0 {
		agents, err := h.agentService.AgentRepo.FindByIdIn(ctx, agentIds)
		if err != nil {
			return err
		}

		agentNameMap := make(map[string]string)
		for _, agent := range agents {
			agentNameMap[agent.ID] = agent.Name
		}

		for i, rule := range page.Items {
			for _, agentId := range rule.AgentIds {
				page.Items[i].AgentNames = append(page.Items[i].AgentNames, agentNameMap[agentId])
			}
		}
	}

	return orz.Ok(c, page)
}

func (h *AlertRuleHandler) Create(c *echo.Context) error {
	var req service.AlertRuleRequest
	if err := c.Bind(&req); err != nil {
		return orz.NewError(400, "请求参数错误")
	}

	item, err := h.ruleService.CreateRule(c.Request().Context(), &req)
	if err != nil {
		return err
	}
	return orz.Ok(c, item)
}

func (h *AlertRuleHandler) Update(c *echo.Context) error {
	id := c.Param("id")

	var req service.AlertRuleRequest
	if err := c.Bind(&req); err != nil {
		return orz.NewError(400, "请求参数错误")
	}

	item, err := h.ruleService.UpdateRule(c.Request().Context(), id, &req)
	if err != nil {
		return err
	}
	return orz.Ok(c, item)
}

func (h *AlertRuleHandler) Delete(c *echo.Context) error {
	id := c.Param("id")

	if err := h.ruleService.DeleteRule(c.Request().Context(), id); err != nil {
		return err
	}
	return nil
}

// Enable 启用告警规则
func (h *AlertRuleHandler) Enable(c *echo.Context) error {
	id := c.Param("id")

	if err := h.ruleService.UpdateRuleEnabled(c.Request().Context(), id, true); err != nil {
		return err
	}
	return orz.Ok(c, orz.Map{})
}

// Disable 停用告警规则
func (h *AlertRuleHandler) Disable(c *echo.Context) error {
	id := c.Param("id")

	if err := h.ruleService.UpdateRuleEnabled(c.Request().Context(), id, false); err != nil {
		return err
	}
	return orz.Ok(c, orz.Map{})
}
