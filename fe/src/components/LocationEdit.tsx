import {
  Edit,
  SimpleForm,
  TextInput,
  required,
} from 'react-admin';
import React from 'react';
import ReadOnlyReferenceArrayInput from './ReadOnlyReferenceArrayInput';

function LocationEdit() {
  return (
    <Edit>
      <SimpleForm>
        <TextInput source="name" validate={[required()]} label="Name" fullWidth name="name" />
        <TextInput source="address" multiline label="Address" name="address" />
        <ReadOnlyReferenceArrayInput source="areas" reference="areas" label="Areas" name="areas" />
      </SimpleForm>
    </Edit>
  );
}

export default LocationEdit;
