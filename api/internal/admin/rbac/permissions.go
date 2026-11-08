package rbac

import (
	"fmt"
	"net/http"
)

type Permission string

const (
	PermAuthenticated   Permission = "admin:authenticated"
	PermOrdersRead      Permission = "orders:read"
	PermOrdersWrite     Permission = "orders:write"
	PermProductsRead    Permission = "products:read"
	PermProductsWrite   Permission = "products:write"
	PermProductsDelete  Permission = "products:delete"
	PermInventoryAdjust Permission = "inventory:adjust"
	PermCustomersRead   Permission = "customers:read"
	PermCustomersWrite  Permission = "customers:write"
	PermVouchersManage  Permission = "vouchers:manage"
	PermContentManage   Permission = "content:manage"
	PermMessagesManage  Permission = "messages:manage"
	PermReportsView     Permission = "reports:view"
	PermFinanceView     Permission = "finance:view"
	PermStaffManage     Permission = "staff:manage"
	PermSettingsManage  Permission = "settings:manage"
	PermAuditView       Permission = "audit:view"
)

var AllPermissions = []Permission{
	PermOrdersRead,
	PermOrdersWrite,
	PermProductsRead,
	PermProductsWrite,
	PermProductsDelete,
	PermInventoryAdjust,
	PermCustomersRead,
	PermCustomersWrite,
	PermVouchersManage,
	PermContentManage,
	PermMessagesManage,
	PermReportsView,
	PermFinanceView,
	PermStaffManage,
	PermSettingsManage,
	PermAuditView,
}

var StaffPermissions = []Permission{
	PermOrdersRead,
	PermOrdersWrite,
	PermProductsRead,
	PermProductsWrite,
	PermInventoryAdjust,
	PermCustomersRead,
	PermMessagesManage,
}

func PermissionsForRole(role string) []Permission {
	switch role {
	case "admin":
		out := make([]Permission, len(AllPermissions))
		copy(out, AllPermissions)
		return out
	case "staff":
		out := make([]Permission, len(StaffPermissions))
		copy(out, StaffPermissions)
		return out
	default:
		return nil
	}
}

func PermissionStringsForRole(role string) []string {
	perms := PermissionsForRole(role)
	out := make([]string, len(perms))
	for i, p := range perms {
		out[i] = string(p)
	}
	return out
}

func HasPermission(role string, perm Permission) bool {
	if perm == PermAuthenticated {
		return role == "admin" || role == "staff"
	}
	for _, p := range PermissionsForRole(role) {
		if p == perm {
			return true
		}
	}
	return false
}

type RouteRequirement struct {
	Method     string
	Pattern    string
	Permission Permission
}

// RoutePermissions is the single source of truth mapping every protected /v1/admin/* route to its required permission.
var RoutePermissions = []RouteRequirement{
	// Self / session / counters / search
	{http.MethodGet, "/v1/admin/me", PermAuthenticated},
	{http.MethodPatch, "/v1/admin/me", PermAuthenticated},
	{http.MethodPut, "/v1/admin/me/password", PermAuthenticated},
	{http.MethodPost, "/v1/admin/me/2fa/setup", PermAuthenticated},
	{http.MethodPost, "/v1/admin/me/2fa/confirm", PermAuthenticated},
	{http.MethodPost, "/v1/admin/me/2fa/disable", PermAuthenticated},
	{http.MethodGet, "/v1/admin/me/sessions", PermAuthenticated},
	{http.MethodDelete, "/v1/admin/me/sessions/{id}", PermAuthenticated},
	{http.MethodGet, "/v1/admin/counters", PermAuthenticated},
	{http.MethodGet, "/v1/admin/search", PermAuthenticated},

	// Dashboard
	{http.MethodGet, "/v1/admin/dashboard", PermReportsView},

	// Orders
	{http.MethodGet, "/v1/admin/orders", PermOrdersRead},
	{http.MethodPost, "/v1/admin/orders", PermOrdersWrite},
	{http.MethodGet, "/v1/admin/orders/export.csv", PermOrdersRead},
	{http.MethodPost, "/v1/admin/orders/bulk-status", PermOrdersWrite},
	{http.MethodGet, "/v1/admin/orders/{orderNo}", PermOrdersRead},
	{http.MethodPatch, "/v1/admin/orders/{orderNo}", PermOrdersWrite},
	{http.MethodPost, "/v1/admin/orders/{orderNo}/status", PermOrdersWrite},
	{http.MethodPut, "/v1/admin/orders/{orderNo}/items", PermOrdersWrite},
	{http.MethodGet, "/v1/admin/orders/{orderNo}/whatsapp", PermOrdersRead},

	// Products, categories, brands, uploads
	{http.MethodGet, "/v1/admin/products", PermProductsRead},
	{http.MethodPost, "/v1/admin/products", PermProductsWrite},
	{http.MethodPost, "/v1/admin/products/bulk", PermProductsWrite},
	{http.MethodPost, "/v1/admin/products/import", PermProductsWrite},
	{http.MethodGet, "/v1/admin/products/export.csv", PermProductsRead},
	{http.MethodGet, "/v1/admin/products/import/template.csv", PermProductsRead},
	{http.MethodGet, "/v1/admin/products/{id}", PermProductsRead},
	{http.MethodPatch, "/v1/admin/products/{id}", PermProductsWrite},
	{http.MethodDelete, "/v1/admin/products/{id}", PermProductsDelete},
	{http.MethodPost, "/v1/admin/products/{id}/duplicate", PermProductsWrite},
	{http.MethodPost, "/v1/admin/uploads", PermProductsWrite},
	{http.MethodDelete, "/v1/admin/uploads/{id}", PermProductsWrite},
	{http.MethodGet, "/v1/admin/categories", PermProductsRead},
	{http.MethodPost, "/v1/admin/categories", PermProductsWrite},
	{http.MethodPost, "/v1/admin/categories/reorder", PermProductsWrite},
	{http.MethodPatch, "/v1/admin/categories/{id}", PermProductsWrite},
	{http.MethodDelete, "/v1/admin/categories/{id}", PermProductsWrite},
	{http.MethodGet, "/v1/admin/brands", PermProductsRead},
	{http.MethodPost, "/v1/admin/brands", PermProductsWrite},
	{http.MethodPatch, "/v1/admin/brands/{id}", PermProductsWrite},
	{http.MethodDelete, "/v1/admin/brands/{id}", PermProductsWrite},

	// Inventory
	{http.MethodGet, "/v1/admin/inventory", PermInventoryAdjust},
	{http.MethodPost, "/v1/admin/inventory/adjust", PermInventoryAdjust},
	{http.MethodPost, "/v1/admin/inventory/restock", PermInventoryAdjust},
	{http.MethodPost, "/v1/admin/inventory/opname", PermInventoryAdjust},
	{http.MethodGet, "/v1/admin/inventory/movements", PermInventoryAdjust},
	{http.MethodGet, "/v1/admin/inventory/movements/export.csv", PermInventoryAdjust},
	{http.MethodGet, "/v1/admin/inventory/valuation", PermFinanceView},

	// Customers
	{http.MethodGet, "/v1/admin/customers", PermCustomersRead},
	{http.MethodGet, "/v1/admin/customers/export.csv", PermCustomersRead},
	{http.MethodGet, "/v1/admin/customers/{id}", PermCustomersRead},
	{http.MethodPatch, "/v1/admin/customers/{id}", PermCustomersWrite},
	{http.MethodPost, "/v1/admin/customers/{id}/deactivate", PermCustomersWrite},
	{http.MethodPost, "/v1/admin/customers/{id}/reactivate", PermCustomersWrite},
	{http.MethodPost, "/v1/admin/customers/{id}/force-password-reset", PermCustomersWrite},

	// Vouchers
	{http.MethodGet, "/v1/admin/vouchers", PermVouchersManage},
	{http.MethodPost, "/v1/admin/vouchers", PermVouchersManage},
	{http.MethodPatch, "/v1/admin/vouchers/{id}", PermVouchersManage},
	{http.MethodDelete, "/v1/admin/vouchers/{id}", PermVouchersManage},
	{http.MethodGet, "/v1/admin/vouchers/{id}/redemptions", PermVouchersManage},

	// Content (Announcements, Slides, FAQs)
	{http.MethodGet, "/v1/admin/announcements", PermContentManage},
	{http.MethodPost, "/v1/admin/announcements", PermContentManage},
	{http.MethodPost, "/v1/admin/announcements/reorder", PermContentManage},
	{http.MethodPatch, "/v1/admin/announcements/{id}", PermContentManage},
	{http.MethodDelete, "/v1/admin/announcements/{id}", PermContentManage},
	{http.MethodGet, "/v1/admin/slides", PermContentManage},
	{http.MethodPost, "/v1/admin/slides", PermContentManage},
	{http.MethodPost, "/v1/admin/slides/reorder", PermContentManage},
	{http.MethodPatch, "/v1/admin/slides/{id}", PermContentManage},
	{http.MethodDelete, "/v1/admin/slides/{id}", PermContentManage},
	{http.MethodGet, "/v1/admin/faqs", PermContentManage},
	{http.MethodPost, "/v1/admin/faqs", PermContentManage},
	{http.MethodPost, "/v1/admin/faqs/reorder", PermContentManage},
	{http.MethodPatch, "/v1/admin/faqs/{id}", PermContentManage},
	{http.MethodDelete, "/v1/admin/faqs/{id}", PermContentManage},

	// Messages & Subscribers
	{http.MethodGet, "/v1/admin/messages", PermMessagesManage},
	{http.MethodPatch, "/v1/admin/messages/{id}", PermMessagesManage},
	{http.MethodDelete, "/v1/admin/messages/{id}", PermMessagesManage},
	{http.MethodGet, "/v1/admin/subscribers", PermMessagesManage},
	{http.MethodGet, "/v1/admin/subscribers/export.csv", PermMessagesManage},
	{http.MethodDelete, "/v1/admin/subscribers/{id}", PermMessagesManage},

	// Reports
	{http.MethodGet, "/v1/admin/reports/{name}", PermReportsView},
	{http.MethodGet, "/v1/admin/reports/{name}/export.csv", PermReportsView},

	// Staff
	{http.MethodGet, "/v1/admin/staff", PermStaffManage},
	{http.MethodPost, "/v1/admin/staff", PermStaffManage},
	{http.MethodPatch, "/v1/admin/staff/{id}", PermStaffManage},
	{http.MethodPost, "/v1/admin/staff/{id}/deactivate", PermStaffManage},
	{http.MethodPost, "/v1/admin/staff/{id}/reactivate", PermStaffManage},
	{http.MethodPost, "/v1/admin/staff/{id}/force-logout", PermStaffManage},
	{http.MethodPost, "/v1/admin/staff/{id}/reset-2fa", PermStaffManage},

	// Audit Log
	{http.MethodGet, "/v1/admin/audit-log", PermAuditView},
	{http.MethodGet, "/v1/admin/audit-log/export.csv", PermAuditView},

	// Settings
	{http.MethodGet, "/v1/admin/settings", PermSettingsManage},
	{http.MethodGet, "/v1/admin/settings/{group}", PermSettingsManage},
	{http.MethodPut, "/v1/admin/settings/{group}", PermSettingsManage},
}

func LookupRoutePermission(method, pattern string) (Permission, error) {
	for _, r := range RoutePermissions {
		if r.Method == method && r.Pattern == pattern {
			return r.Permission, nil
		}
	}
	return "", fmt.Errorf("no permission registered for %s %s", method, pattern)
}
