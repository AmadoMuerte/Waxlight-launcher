import type { LucideIcon } from "lucide-react";
import type { ReactNode } from "react";
import { NavLink } from "react-router";

export function NavItem({
  icon: Icon,
  indicator = false,
  label,
  to,
}: {
  icon: LucideIcon;
  indicator?: boolean;
  label: ReactNode;
  to: string;
}) {
  return (
    <NavLink to={to} className={({ isActive }) => `navItem${isActive ? " active" : ""}`}>
      <Icon className="navItemIcon" size={17} strokeWidth={1.8} aria-hidden="true" />
      <span>{label}</span>
      {indicator && <span className="navPulse" aria-hidden="true" />}
    </NavLink>
  );
}
