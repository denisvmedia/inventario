import {
  List,
  Datagrid,
  TextField,
} from 'react-admin';
import React from 'react';

function CommodityList() {
  return (
    <List>
      <Datagrid rowClick="edit">
        <TextField source="short_name" />
        <TextField source="type" />
        <TextField source="count" />
        <TextField source="original_price" />
        <TextField source="current_price" />
        <TextField source="status" />
        <TextField source="purchase_date" />
      </Datagrid>
    </List>
  );
}

export default CommodityList;
