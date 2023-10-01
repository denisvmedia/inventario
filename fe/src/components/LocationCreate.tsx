import {
  Create,
  SimpleForm,
  TextInput,
  required,
} from 'react-admin';
import React from 'react';

function LocationCreate() {
  return (
    <Create>
      <SimpleForm>
        <TextInput source="name" validate={[required()]} label="Name" fullWidth name="name" />
        <TextInput source="address" multiline label="Address" name="address" />
      </SimpleForm>
    </Create>
  );
}

export default LocationCreate;
