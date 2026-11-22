import React from "react";

type IconProps = React.SVGProps<SVGSVGElement> & { size?: number };

function Icon({ size = 16, children, className = "", ...props }: IconProps & { children: React.ReactNode }) {
  return (
    <svg
      xmlns="http://www.w3.org/2000/svg"
      width={size}
      height={size}
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth={1.75}
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
      className={`shrink-0 ${className}`}
      {...props}
    >
      {children}
    </svg>
  );
}

export const ShieldIcon = (p: IconProps) => (
  <Icon {...p}><path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z" /></Icon>
);

export const DashboardIcon = (p: IconProps) => (
  <Icon {...p}><rect width="7" height="9" x="3" y="3" rx="1" /><rect width="7" height="5" x="14" y="3" rx="1" /><rect width="7" height="9" x="14" y="12" rx="1" /><rect width="7" height="5" x="3" y="16" rx="1" /></Icon>
);

export const BellIcon = (p: IconProps) => (
  <Icon {...p}><path d="M6 8a6 6 0 0 1 12 0c0 7 3 9 3 9H3s3-2 3-9" /><path d="M10.3 21a1.94 1.94 0 0 0 3.4 0" /></Icon>
);

export const ZapIcon = (p: IconProps) => (
  <Icon {...p}><polygon points="13 2 3 14 12 14 11 22 21 10 12 10 13 2" /></Icon>
);

export const BriefcaseIcon = (p: IconProps) => (
  <Icon {...p}><path d="M16 20V4a2 2 0 0 0-2-2h-4a2 2 0 0 0-2 2v16" /><rect width="20" height="14" x="2" y="6" rx="2" /></Icon>
);

export const TerminalIcon = (p: IconProps) => (
  <Icon {...p}><polyline points="4 17 10 11 4 5" /><line x1="12" x2="20" y1="19" y2="19" /></Icon>
);

export const ShieldAlertIcon = (p: IconProps) => (
  <Icon {...p}><path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z" /><path d="M12 8v4" /><path d="M12 16h.01" /></Icon>
);

export const ServerIcon = (p: IconProps) => (
  <Icon {...p}><rect width="20" height="8" x="2" y="2" rx="2" ry="2" /><rect width="20" height="8" x="2" y="14" rx="2" ry="2" /><line x1="6" x2="6.01" y1="6" y2="6" /><line x1="6" x2="6.01" y1="18" y2="18" /></Icon>
);

export const UsersIcon = (p: IconProps) => (
  <Icon {...p}><path d="M16 21v-2a4 4 0 0 0-4-4H6a4 4 0 0 0-4 4v2" /><circle cx="9" cy="7" r="4" /><path d="M22 21v-2a4 4 0 0 0-3-3.87" /><path d="M16 3.13a4 4 0 0 1 0 7.75" /></Icon>
);

export const ActivityIcon = (p: IconProps) => (
  <Icon {...p}><polyline points="22 12 18 12 15 21 9 3 6 12 2 12" /></Icon>
);

export const SearchIcon = (p: IconProps) => (
  <Icon {...p}><circle cx="11" cy="11" r="8" /><path d="m21 21-4.3-4.3" /></Icon>
);

export const RefreshIcon = (p: IconProps) => (
  <Icon {...p}><path d="M3 12a9 9 0 0 1 9-9 9.75 9.75 0 0 1 6.74 2.74L21 8" /><path d="M21 3v5h-5" /><path d="M21 12a9 9 0 0 1-9 9 9.75 9.75 0 0 1-6.74-2.74L3 16" /><path d="M8 16H3v5" /></Icon>
);

export const ChevronLeftIcon = (p: IconProps) => (
  <Icon {...p}><path d="m15 18-6-6 6-6" /></Icon>
);

export const ChevronRightIcon = (p: IconProps) => (
  <Icon {...p}><path d="m9 18 6-6-6-6" /></Icon>
);

export const ChevronDownIcon = (p: IconProps) => (
  <Icon {...p}><path d="m6 9 6 6 6-6" /></Icon>
);

export const LogOutIcon = (p: IconProps) => (
  <Icon {...p}><path d="M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4" /><polyline points="16 17 21 12 16 7" /><line x1="21" x2="9" y1="12" y2="12" /></Icon>
);

export const LogInIcon = (p: IconProps) => (
  <Icon {...p}><path d="M15 3h4a2 2 0 0 1 2 2v14a2 2 0 0 1-2 2h-4" /><polyline points="10 17 15 12 10 7" /><line x1="15" x2="3" y1="12" y2="12" /></Icon>
);

export const XIcon = (p: IconProps) => (
  <Icon {...p}><path d="M18 6 6 18" /><path d="m6 6 12 12" /></Icon>
);

export const CheckIcon = (p: IconProps) => (
  <Icon {...p}><path d="M20 6 9 17l-5-5" /></Icon>
);

export const CopyIcon = (p: IconProps) => (
  <Icon {...p}><rect width="14" height="14" x="8" y="8" rx="2" ry="2" /><path d="M4 16c-1.1 0-2-.9-2-2V4c0-1.1.9-2 2-2h10c1.1 0 2 .9 2 2" /></Icon>
);

export const DownloadIcon = (p: IconProps) => (
  <Icon {...p}><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4" /><polyline points="7 10 12 15 17 10" /><line x1="12" x2="12" y1="15" y2="3" /></Icon>
);

export const EyeIcon = (p: IconProps) => (
  <Icon {...p}><path d="M2 12s3-7 10-7 10 7 10 7-3 7-10 7-10-7-10-7Z" /><circle cx="12" cy="12" r="3" /></Icon>
);

export const EyeOffIcon = (p: IconProps) => (
  <Icon {...p}><path d="M9.88 9.88a3 3 0 1 0 4.24 4.24" /><path d="M10.73 5.08A10.43 10.43 0 0 1 12 5c7 0 10 7 10 7a13.16 13.16 0 0 1-1.67 2.68" /><path d="M6.61 6.61A13.526 13.526 0 0 0 2 12s3 7 10 7a9.74 9.74 0 0 0 5.39-1.61" /><line x1="2" x2="22" y1="2" y2="22" /></Icon>
);

export const PlusIcon = (p: IconProps) => (
  <Icon {...p}><path d="M5 12h14" /><path d="M12 5v14" /></Icon>
);

export const TrashIcon = (p: IconProps) => (
  <Icon {...p}><path d="M3 6h18" /><path d="M19 6v14c0 1-1 2-2 2H7c-1 0-2-1-2-2V6" /><path d="M8 6V4c0-1 1-2 2-2h4c1 0 2 1 2 2v2" /></Icon>
);

export const FlaskIcon = (p: IconProps) => (
  <Icon {...p}><path d="M10 2v7.31" /><path d="M14 9.3V1.99" /><path d="M8.5 2h7" /><path d="M14 9.3a6.5 6.5 0 1 1-4 0" /><path d="M5.52 16h12.96" /></Icon>
);

export const ArrowUpRightIcon = (p: IconProps) => (
  <Icon {...p}><path d="M7 7h10v10" /><path d="M7 17 17 7" /></Icon>
);

export const AlertTriangleIcon = (p: IconProps) => (
  <Icon {...p}><path d="m21.73 18-8-14a2 2 0 0 0-3.48 0l-8 14A2 2 0 0 0 4 21h16a2 2 0 0 0 1.73-3Z" /><path d="M12 9v4" /><path d="M12 17h.01" /></Icon>
);

export const CheckCircleIcon = (p: IconProps) => (
  <Icon {...p}><path d="M22 11.08V12a10 10 0 1 1-5.93-9.14" /><polyline points="22 4 12 14.01 9 11.01" /></Icon>
);

export const MonitorIcon = (p: IconProps) => (
  <Icon {...p}><rect width="20" height="14" x="2" y="3" rx="2" /><line x1="8" x2="16" y1="21" y2="21" /><line x1="12" x2="12" y1="17" y2="21" /></Icon>
);

export const LockIcon = (p: IconProps) => (
  <Icon {...p}><rect width="18" height="11" x="3" y="11" rx="2" ry="2" /><path d="M7 11V7a5 5 0 0 1 10 0v4" /></Icon>
);

export const UnlockIcon = (p: IconProps) => (
  <Icon {...p}><rect width="18" height="11" x="3" y="11" rx="2" ry="2" /><path d="M7 11V7a5 5 0 0 1 9.9-1" /></Icon>
);
