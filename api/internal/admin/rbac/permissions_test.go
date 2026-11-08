package rbac

import (
	"testing"
)

func TestPermissionMatrix(t *testing.T) {
	// Admin has every permission
	for _, p := range AllPermissions {
		if !HasPermission("admin", p) {
			t.Errorf("expected admin to have permission %s", p)
		}
	}
	if !HasPermission("admin", PermAuthenticated) {
		t.Errorf("expected admin to have PermAuthenticated")
	}

	// Staff has operational permissions
	staffAllowed := []Permission{
		PermAuthenticated,
		PermOrdersRead,
		PermOrdersWrite,
		PermProductsRead,
		PermProductsWrite,
		PermInventoryAdjust,
		PermCustomersRead,
		PermMessagesManage,
	}
	for _, p := range staffAllowed {
		if !HasPermission("staff", p) {
			t.Errorf("expected staff to have permission %s", p)
		}
	}

	// Staff MUST NOT have sensitive admin-only permissions
	staffForbidden := []Permission{
		PermProductsDelete,
		PermCustomersWrite,
		PermVouchersManage,
		PermContentManage,
		PermReportsView,
		PermFinanceView,
		PermStaffManage,
		PermSettingsManage,
		PermAuditView,
	}
	for _, p := range staffForbidden {
		if HasPermission("staff", p) {
			t.Errorf("expected staff NOT to have permission %s", p)
		}
	}

	// Customer has no admin permissions
	for _, p := range AllPermissions {
		if HasPermission("customer", p) {
			t.Errorf("expected customer NOT to have permission %s", p)
		}
	}
	if HasPermission("customer", PermAuthenticated) {
		t.Errorf("expected customer NOT to have PermAuthenticated")
	}
}

func TestRoutePermissionsCompleteness(t *testing.T) {
	if len(RoutePermissions) < 60 {
		t.Fatalf("expected at least 60 route permission mappings, got %d", len(RoutePermissions))
	}
	for _, req := range RoutePermissions {
		if !HasPermission("admin", req.Permission) {
			t.Errorf("route %s %s has unknown permission %s", req.Method, req.Pattern, req.Permission)
		}
		got, err := LookupRoutePermission(req.Method, req.Pattern)
		if err != nil || got != req.Permission {
			t.Errorf("LookupRoutePermission(%s, %s) = %s, %v", req.Method, req.Pattern, got, err)
		}
	}
}
