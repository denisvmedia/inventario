import {
  List,
  Datagrid,
  TextField,
  ReferenceField,
} from 'react-admin';
import React from 'react';

function CommodityList() {
  return (
    <List>
      <Datagrid rowClick="edit">
        <TextField source="name" />
        <ReferenceField source="commodity_id" reference="commodities" />
      </Datagrid>
    </List>
  );
}

export default CommodityList;
