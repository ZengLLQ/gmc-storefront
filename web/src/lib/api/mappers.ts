import type { components } from "./schema";

export type PublicSiteData = components["schemas"]["PublicSiteResponse"];
export type CategoryDTO = components["schemas"]["Category"];
export type BrandDTO = components["schemas"]["Brand"];
export type ImageDTO = components["schemas"]["Image"];
export type ProductCardDTO = components["schemas"]["ProductCard"];
export type ProductDetailDTO = components["schemas"]["ProductDetail"];
export type ProductVariantDTO = components["schemas"]["ProductVariant"];
export type ProductOptionDTO = components["schemas"]["ProductOption"];
export type ProductOptionValueDTO = components["schemas"]["ProductOptionValue"];
export type ProductListResponseDTO =
  components["schemas"]["ProductListResponse"];
export type ProductFacetsDTO = components["schemas"]["ProductFacets"];

export type CartItemInputDTO = components["schemas"]["CartItemInput"];
export type CartLineDTO = components["schemas"]["CartLine"];
export type CartValidationResponseDTO =
  components["schemas"]["CartValidationResponse"];

export type CheckoutRequestDTO = components["schemas"]["CheckoutRequest"];
export type CheckoutResponseDTO = components["schemas"]["CheckoutResponse"];
export type OrderDTO = components["schemas"]["Order"];
export type OrderItemDTO = components["schemas"]["OrderItem"];
export type OrderSummaryDTO = components["schemas"]["OrderSummary"];
export type OrderSummaryListResponseDTO =
  components["schemas"]["OrderSummaryListResponse"];
export type ShippingAddressDTO = components["schemas"]["ShippingAddress"];

export type UserDTO = components["schemas"]["User"];
export type AddressDTO = components["schemas"]["Address"];
export type AddressInputDTO = components["schemas"]["AddressInput"];

// Admin DTOs
export type AdminUserDTO = components["schemas"]["AdminStaffUser"];
export type AdminLoginResponseDTO = components["schemas"]["AdminAuthResponse"];
export type AdminCountersDTO = components["schemas"]["AdminCounters"];
export type AdminSearchResponseDTO =
  components["schemas"]["AdminSearchResponse"];
export type AdminDashboardDTO = components["schemas"]["AdminDashboardResponse"];
export type AdminOrderRowDTO = components["schemas"]["AdminOrderRow"];
export type AdminOrderListResponseDTO =
  components["schemas"]["AdminOrderListResponse"];
export type AdminOrderDTO = components["schemas"]["AdminOrder"];
export type AdminOrderItemDTO = components["schemas"]["AdminOrderItem"];
export type AdminProductRowDTO = components["schemas"]["AdminProductRow"];
export type AdminProductListResponseDTO =
  components["schemas"]["AdminProductListResponse"];
export type AdminProductDetailDTO = components["schemas"]["AdminProduct"];
export type AdminProductWriteDTO = components["schemas"]["AdminProductInput"];
export type AdminCategoryDTO = components["schemas"]["AdminCategory"];
export type AdminBrandDTO = components["schemas"]["AdminBrand"];
export type AdminInventoryItemDTO = components["schemas"]["AdminInventoryItem"];
export type AdminInventoryListResponseDTO =
  components["schemas"]["AdminInventoryListResponse"];
export type AdminStockMovementDTO = components["schemas"]["AdminStockMovement"];
export type AdminStockMovementListResponseDTO =
  components["schemas"]["AdminStockMovementListResponse"];
export type AdminInventoryValuationDTO =
  components["schemas"]["AdminInventoryValuation"];
export type AdminCustomerRowDTO = components["schemas"]["AdminCustomerRow"];
export type AdminCustomerListResponseDTO =
  components["schemas"]["AdminCustomerListResponse"];
export type AdminCustomerDetailDTO =
  components["schemas"]["AdminCustomerDetail"];
export type AdminVoucherDTO = components["schemas"]["AdminVoucher"];
export type AdminVoucherInputDTO = components["schemas"]["AdminVoucherInput"];
export type AdminVoucherRedemptionDTO =
  components["schemas"]["AdminVoucherRedemption"];
export type AdminAnnouncementDTO = components["schemas"]["AdminAnnouncement"];
export type AdminPromoSlideDTO = components["schemas"]["AdminPromoSlide"];
export type AdminFaqDTO = components["schemas"]["AdminFaq"];
export type AdminMessageDTO = components["schemas"]["AdminMessage"];
export type AdminMessageListResponseDTO =
  components["schemas"]["AdminMessageListResponse"];
export type AdminSubscriberDTO = components["schemas"]["AdminSubscriber"];
export type AdminSubscriberListResponseDTO =
  components["schemas"]["AdminSubscriberListResponse"];
export type AdminReportResponseDTO =
  components["schemas"]["AdminReportResponse"];
export type AdminStaffUserDTO = components["schemas"]["AdminStaffUser"];
export type AdminAuditLogEntryDTO = components["schemas"]["AdminAuditLogEntry"];
export type AdminAuditLogListResponseDTO =
  components["schemas"]["AdminAuditLogListResponse"];
export type AdminSettingsAllResponseDTO =
  components["schemas"]["AdminSettingsAllResponse"];
export type AdminSettingsGroupPayloadDTO =
  components["schemas"]["AdminSettingsGroupPayload"];
export type AdminSessionDTO = components["schemas"]["AdminSessionInfo"];
export type AdminImportDryRunResponseDTO =
  components["schemas"]["AdminProductImportResponse"];
