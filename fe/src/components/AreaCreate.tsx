import {
  ReferenceInput,
  Create,
  SimpleForm,
  TextInput,
  SelectInput,
  required,
} from 'react-admin';
import React from 'react';

function AreaCreate() {
  return (
    <Create>
      <SimpleForm>
        <TextInput source="name" validate={[required()]} label="Name" fullWidth name="name" />
        <ReferenceInput source="location_id" reference="locations" name="location" />
      </SimpleForm>
    </Create>
  );
}

export default AreaCreate;
