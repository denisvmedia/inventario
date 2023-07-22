import React, { useState } from 'react';
import api from '../api/api';
import Errors from './Errors';

// interface AddLocationFormProps {
//   onLocationAdded: () => void;
// }

function AddLocationForm({ onLocationAdded }:any) {
  const [name, setName] = useState('');
  const [address, setAddress] = useState('');
  const [errors, setErrors] = useState<object[]>([]);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();

    try {
      /* const response = */ await api.post('/locations', {
        data: {
          name,
          address,
        },
      });

      setName('');
      setAddress('');
      setErrors([]);
      onLocationAdded(); // Invoke the callback function to trigger re-fetching of locations
    } catch (error: any) {
      if (error.response && error.response.data && error.response.data.errors && error.response.data.errors.length > 0) {
        // example of error.response.data.errors:
        // [
        //   {
        //     error: {
        //       error: {
        //         data: {
        //           name: 'cannot be blank',
        //           address: 'cannot be blank'
        //         }
        //       },
        //       type: 'validation.Errors'
        //     }
        //     status: 'Unprocessable Entity'
        //   }
        // ]
        setErrors(error.response.data.errors);
      } else {
        // eslint-disable-next-line no-console
        console.log(error.response.data);
        setErrors([{
          status: 'Unknown Error',
        }]);
      }
    }
  };

  return (
    <div>
      <h2>Add Location</h2>
      <Errors errors={errors} />
      <form onSubmit={handleSubmit}>
        <div>
          {/* eslint-disable-next-line jsx-a11y/label-has-associated-control */}
          <label htmlFor="name">Name:</label>
          <input type="text" id="name" value={name} onChange={(e) => setName(e.target.value)} />
        </div>
        <div>
          {/* eslint-disable-next-line jsx-a11y/label-has-associated-control */}
          <label htmlFor="address">Address:</label>
          <input type="text" id="address" value={address} onChange={(e) => setAddress(e.target.value)} />
        </div>
        <button type="submit">Add Location</button>
      </form>
    </div>
  );
}

export default AddLocationForm;
