import {
  List,
  Datagrid,
  TextField,
  ReferenceField,
} from 'react-admin';
import React from 'react';

function AreaList() {
  return (
    <List>
      <Datagrid rowClick="edit">
        <TextField source="name" />
        <ReferenceField source="location_id" reference="locations" />
      </Datagrid>
    </List>
  );
}

export default AreaList;
