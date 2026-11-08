import React from "react";
import { AnnouncementBar } from "@/components/store/AnnouncementBar";
import { StoreHeader } from "@/components/store/StoreHeader";
import { CartDrawer } from "@/components/store/CartDrawer";
import { StoreFooter } from "@/components/store/StoreFooter";

export default function StoreLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <div className="flex flex-col min-h-screen bg-bg">
      <AnnouncementBar />
      <StoreHeader />
      <CartDrawer />
      <main className="flex-1">{children}</main>
      <StoreFooter />
    </div>
  );
}
