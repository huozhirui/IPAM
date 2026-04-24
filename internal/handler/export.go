package handler

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"network-plan/internal/store"

	"github.com/gin-gonic/gin"
)

// ExportHandler 数据导出相关 API
type ExportHandler struct {
	allocRepo *store.AllocRepo
	auditRepo *store.AuditRepo
}

// NewExportHandler 创建导出 Handler
func NewExportHandler(ar *store.AllocRepo, aur *store.AuditRepo) *ExportHandler {
	return &ExportHandler{allocRepo: ar, auditRepo: aur}
}

// Export 导出分配记录或审计日志
// GET /api/export?format=csv&type=audit
//   type: allocation(默认) / audit
//   format: csv(默认) / json
func (h *ExportHandler) Export(c *gin.Context) {
	format := c.DefaultQuery("format", "csv")
	dataType := c.DefaultQuery("type", "allocation")

	switch dataType {
	case "audit":
		h.exportAudit(c, format)
	default:
		h.exportAllocations(c, format)
	}
}

func (h *ExportHandler) exportAllocations(c *gin.Context, format string) {
	allocs, err := h.allocRepo.ListAll()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	switch format {
	case "csv":
		c.Header("Content-Type", "text/csv; charset=utf-8")
		c.Header("Content-Disposition", "attachment; filename=allocations.csv")
		w := csv.NewWriter(c.Writer)
		w.Write([]string{"ID", "PoolID", "CIDR", "申请IP数", "实际IP数", "用途", "负责人", "分配时间"})
		for _, a := range allocs {
			w.Write([]string{
				fmt.Sprintf("%d", a.ID),
				fmt.Sprintf("%d", a.PoolID),
				a.CIDR,
				fmt.Sprintf("%d", a.IPCount),
				fmt.Sprintf("%d", a.ActualCount),
				a.Purpose,
				a.AllocatedBy,
				a.AllocatedAt.Format("2006-01-02 15:04:05"),
			})
		}
		w.Flush()
	default:
		c.Header("Content-Type", "application/json; charset=utf-8")
		c.Header("Content-Disposition", "attachment; filename=allocations.json")
		data, _ := json.MarshalIndent(allocs, "", "  ")
		c.Writer.Write(data)
	}
}

func (h *ExportHandler) exportAudit(c *gin.Context, format string) {
	logs, err := h.auditRepo.ListAll()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	switch format {
	case "csv":
		c.Header("Content-Type", "text/csv; charset=utf-8")
		c.Header("Content-Disposition", "attachment; filename=audit_logs.csv")
		w := csv.NewWriter(c.Writer)
		w.Write([]string{"ID", "操作类型", "详情", "操作人", "时间"})
		for _, l := range logs {
			w.Write([]string{
				fmt.Sprintf("%d", l.ID),
				l.Action,
				l.Detail,
				l.Operator,
				l.CreatedAt.Format("2006-01-02 15:04:05"),
			})
		}
		w.Flush()
	default:
		c.Header("Content-Type", "application/json; charset=utf-8")
		c.Header("Content-Disposition", "attachment; filename=audit_logs.json")
		data, _ := json.MarshalIndent(logs, "", "  ")
		c.Writer.Write(data)
	}
}
