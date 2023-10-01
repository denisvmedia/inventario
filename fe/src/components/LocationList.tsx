import {
  List,
  Datagrid,
  TextField,
} from 'react-admin';
import React from 'react';

function LocationList() {
  return (
    <List>
      <Datagrid rowClick="edit">
        <TextField source="name" />
        <TextField source="address" />
      </Datagrid>
    </List>
  );
}

export default LocationList;
