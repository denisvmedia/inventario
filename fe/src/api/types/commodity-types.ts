export type CommodityTypes = (
  'white_goods'
  | 'electronics'
  | 'equipment'
  | 'furniture'
  | 'clothes'
  | 'other'
);

export const COMMODITY_TYPE_WHITE_GOODS: CommodityTypes = 'white_goods';
export const COMMODITY_TYPE_ELECTRONICS: CommodityTypes = 'electronics';
export const COMMODITY_TYPE_EQUIPMENT: CommodityTypes = 'equipment';
export const COMMODITY_TYPE_FURNITURE: CommodityTypes = 'furniture';
export const COMMODITY_TYPE_CLOTHES: CommodityTypes = 'clothes';
export const COMMODITY_TYPE_OTHER: CommodityTypes = 'other';

export const COMMODITY_TYPES: CommodityTypes[] = [
  COMMODITY_TYPE_WHITE_GOODS,
  COMMODITY_TYPE_ELECTRONICS,
  COMMODITY_TYPE_EQUIPMENT,
  COMMODITY_TYPE_FURNITURE,
  COMMODITY_TYPE_CLOTHES,
  COMMODITY_TYPE_OTHER,
];

type CommodityTypeElement = { id: string, name: string };

export const COMMODITY_TYPES_ELEMENTS: CommodityTypeElement[] = COMMODITY_TYPES.map((type: CommodityTypes): CommodityTypeElement => ({
  id: type,
  name: type,
}));
