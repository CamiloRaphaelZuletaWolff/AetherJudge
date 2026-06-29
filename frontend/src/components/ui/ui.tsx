// Barrel for the UI kit. Prefer importing from the specific module
// (@/components/ui/button, …) in new code; this keeps a single convenient
// surface and preserves the legacy "@/components/ui/ui" import path.
export { Button, type ButtonProps } from "@/components/ui/button";
export { Card, CardHeader, CardTitle, CardBody, ErrorNotice } from "@/components/ui/card";
export { Input, Textarea, Field } from "@/components/ui/input";
export { Badge, type BadgeVariant } from "@/components/ui/badge";
export { Avatar } from "@/components/ui/avatar";
export { Skeleton } from "@/components/ui/skeleton";
export { Tabs } from "@/components/ui/tabs";
export { Spinner } from "@/components/ui/spinner";
export { ThemeToggle } from "@/components/ui/theme-toggle";
export {
  DropdownMenu,
  DropdownItem,
  DropdownLink,
  DropdownLabel,
  DropdownSeparator,
} from "@/components/ui/dropdown";
export { useToast } from "@/components/ui/toast";
