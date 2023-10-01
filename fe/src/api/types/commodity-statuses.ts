export type CommodityStatuses = (
  'in_use'
  | 'sold'
  | 'lost'
  | 'disposed'
  | 'written_off'
);

export const COMMODITY_STATUS_IN_USE: CommodityStatuses = 'in_use';
export const COMMODITY_STATUS_SOLD: CommodityStatuses = 'sold';
export const COMMODITY_STATUS_LOST: CommodityStatuses = 'lost';
export const COMMODITY_STATUS_DISPOSED: CommodityStatuses = 'disposed';
export const COMMODITY_STATUS_WRITTEN_OFF: CommodityStatuses = 'written_off';

export const COMMODITY_STATUSES: CommodityStatuses[] = [
  COMMODITY_STATUS_IN_USE,
  COMMODITY_STATUS_SOLD,
  COMMODITY_STATUS_LOST,
  COMMODITY_STATUS_DISPOSED,
  COMMODITY_STATUS_WRITTEN_OFF,
];

type CommodityStatusElement = { id: string, name: string };

export const COMMODITY_STATUS_ELEMENTS: CommodityStatusElement[] = COMMODITY_STATUSES.map((type: CommodityStatuses): CommodityStatusElement => ({
  id: type,
  name: type,
}));
