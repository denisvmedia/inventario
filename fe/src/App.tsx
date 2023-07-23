import {
  Admin,
  Resource,
  ReferenceInput,
  EditGuesser,
  ShowGuesser,
  Create,
  List,
  Datagrid,
  TextField,
  SimpleForm,
  TextInput,
  required,
} from 'react-admin';
import React from 'react';
import dataProvider from './dataProvider';

export function LocationCreate() {
  return (
    <Create>
      <SimpleForm>
        <TextInput source="name" validate={[required()]} label="Name" fullWidth name="name" />
        <TextInput source="address" multiline label="Address" name="address" />
      </SimpleForm>
    </Create>
  );
}

export function AreaCreate() {
  return (
    <Create>
      <SimpleForm>
        <TextInput source="name" validate={[required()]} label="Name" fullWidth name="name" />
        <ReferenceInput source="location_id" reference="locations" name="location" />
      </SimpleForm>
    </Create>
  );
}

export function LocationList() {
  return (
    <List>
      <Datagrid rowClick="edit">
        <TextField source="id" />
      </Datagrid>
    </List>
  );
}

export function AreaList() {
  return (
    <List>
      <Datagrid rowClick="edit">
        <TextField source="id" />
      </Datagrid>
    </List>
  );
}

function App() {
  return (
    <Admin dataProvider={dataProvider}>
      <Resource
        name="locations"
        list={LocationList}
        create={LocationCreate}
        edit={EditGuesser}
        show={ShowGuesser}
      />
      <Resource
        name="areas"
        list={AreaList}
        create={AreaCreate}
        edit={EditGuesser}
        show={ShowGuesser}
      />
    </Admin>
  );
}

export default App;
