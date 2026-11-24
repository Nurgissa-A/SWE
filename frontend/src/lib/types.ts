import type { components } from './api-types';

type S = components['schemas'];
export type ApiEvent = S['Event'];
export type TicketType = S['TicketType'];
export type Order = S['Order'];
export type OrderSummary = S['OrderSummary'];
export type Ticket = S['Ticket'];
export type Campaign = S['Campaign'];
export type Checklist = S['ActivationChecklist'];
export type PaymentSetup = S['PaymentSetup'];
export type StaffMember = S['StaffMember'];
export type SupportCase = S['SupportCase'];
export type AnalyticsReport = S['AnalyticsReport'];
export type TimelineEntry = S['TimelineEntry'];
export type OverviewEvent = S['Event'] & { sold?: number; checkedIn?: number; revenueKzt?: number; openCases?: number };
export type AdminRow = Record<string, unknown>;
export type AdminUser = any;